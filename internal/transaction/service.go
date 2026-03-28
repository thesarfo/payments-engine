package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/internal/ledger"
	"github.com/thesarfo/payments-engine/pkg/idempotency"
	"github.com/thesarfo/payments-engine/pkg/logctx"
	"github.com/thesarfo/payments-engine/pkg/logging"
	"github.com/thesarfo/payments-engine/pkg/money"
)

const (
	InternalRail        = "INTERNAL"
	DefaultClearingCode = "GL_LIAB_CLEARING"
	DefaultPostedBy     = "transfer-service"
	defaultIdemTTL      = 24 * time.Hour
	inProgressMarker    = "__IN_PROGRESS__"
)

var (
	ErrInvalidTransfer         = errors.New("invalid transfer request")
	ErrInsufficientFunds       = errors.New("insufficient funds")
	ErrCurrencyMismatch        = errors.New("currency mismatch")
	ErrClearingAccountNotFound = errors.New("clearing account not found")
	ErrTransferInProgress      = errors.New("transfer request is already in progress")
)

type TransferRequest struct {
	IdempotencyKey string
	FromAccountId  uuid.UUID
	ToAccountId    uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	Rail           *string
	Description    *string
}

type TransferService struct {
	repo         Repository
	ledger       *ledger.Ledger
	clearingCode string
	postedBy     string
	idemStore    idempotency.Store
	idemTTL      time.Duration
	logger       zerolog.Logger
}

func NewTransferService(repo Repository, ledgerSvc *ledger.Ledger, stores ...idempotency.Store) *TransferService {
	svc := &TransferService{
		repo:         repo,
		ledger:       ledgerSvc,
		clearingCode: DefaultClearingCode,
		postedBy:     DefaultPostedBy,
		idemTTL:      defaultIdemTTL,
		logger:       logging.New().With().Str("component", "transfer_service").Logger(),
	}
	if len(stores) > 0 {
		svc.idemStore = stores[0]
	}
	return svc
}

// WithClearingCode overrides the default clearing account code.
// Useful in integration tests that create isolated test accounts.
func (s *TransferService) WithClearingCode(code string) *TransferService {
	s.clearingCode = code
	return s
}

func (s *TransferService) GetTransactionByID(ctx context.Context, txID uuid.UUID) (Transaction, error) {
	return s.repo.GetTransactionByID(ctx, txID)
}

func (s *TransferService) idempotencyTransferKey(idempotencyKey string) string {
	return "transfer:" + idempotencyKey
}

// this method returns a stored transaction when the key already holds a completed payload.
// Returns (nil, nil) when there is no store or nothing to return yet.
func (s *TransferService) idempotencyTryReturnCached(ctx context.Context, idemKey string) (*Transaction, error) {
	if s.idemStore == nil {
		return nil, nil
	}
	raw, err := s.idemStore.Get(ctx, idemKey)
	if errors.Is(err, idempotency.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("idempotency get: %w", err)
	}
	if raw == inProgressMarker {
		return nil, ErrTransferInProgress
	}
	tx, err := decodeTransaction(raw)
	if err == nil {
		return &tx, nil
	}
	return nil, nil
}

// this claims processing with SET NX, or returns a duplicate result if another request finished first.
// On success unlock must be called on failure paths; after idempotencyStoreResult, clear the lock flag so defer does not delete the final payload.
func (s *TransferService) idempotencyAcquireExclusive(ctx context.Context, idemKey string) (unlock func(), duplicate *Transaction, err error) {
	if s.idemStore == nil {
		return nil, nil, nil
	}
	ok, err := s.idemStore.SetNX(ctx, idemKey, inProgressMarker, s.idemTTL)
	if err != nil {
		return nil, nil, fmt.Errorf("idempotency setnx: %w", err)
	}
	if !ok {
		raw, err := s.idemStore.Get(ctx, idemKey)
		if err != nil {
			return nil, nil, ErrTransferInProgress
		}
		if raw == inProgressMarker {
			return nil, nil, ErrTransferInProgress
		}
		tx, err := decodeTransaction(raw)
		if err != nil {
			return nil, nil, ErrTransferInProgress
		}
		return nil, &tx, nil
	}
	unlock = func() { _ = s.idemStore.Del(ctx, idemKey) }
	return unlock, nil, nil
}

func (s *TransferService) idempotencyStoreResult(ctx context.Context, idemKey string, tx Transaction) error {
	if s.idemStore == nil {
		return nil
	}
	encoded, err := encodeTransaction(tx)
	if err != nil {
		return fmt.Errorf("encode idempotency payload: %w", err)
	}
	if err := s.idemStore.Set(ctx, idemKey, encoded, s.idemTTL); err != nil {
		return fmt.Errorf("idempotency set result: %w", err)
	}
	return nil
}

func (s *TransferService) Transfer(ctx context.Context, req TransferRequest) (*Transaction, error) {
	traceID := logctx.TraceID(ctx)
	if traceID == "" {
		traceID = "-"
	}

	transferStart := time.Now()
	logFailure := func(transferID string, step string, reason string) {
		s.logger.Warn().
			Str("event", "transfer_failed").
			Str("trace_id", traceID).
			Str("transfer_id", transferID).
			Str("failed_step", step).
			Str("reason", reason).
			Msg("transfer failed")
	}

	if err := validateTransferRequest(req); err != nil {
		logFailure("-", "validate_transfer_request", err.Error())
		return nil, err
	}
	if s.ledger == nil {
		err := fmt.Errorf("%w: ledger service is required", ErrInvalidTransfer)
		logFailure("-", "load_ledger_service", err.Error())
		return nil, err
	}
	idemKey := s.idempotencyTransferKey(req.IdempotencyKey)
	if cached, err := s.idempotencyTryReturnCached(ctx, idemKey); err != nil || cached != nil {
		if err != nil {
			logFailure("-", "idempotency_cache_lookup", err.Error())
		}
		return cached, err
	}
	unlock, dup, err := s.idempotencyAcquireExclusive(ctx, idemKey)
	if err != nil {
		logFailure("-", "idempotency_acquire", err.Error())
		return nil, err
	}
	if dup != nil {
		return dup, nil
	}
	idemLocked := unlock != nil
	if idemLocked {
		defer func() {
			if idemLocked {
				unlock()
			}
		}()
	}

	fromAcc, err := s.repo.GetAccountSnapshot(ctx, req.FromAccountId)
	if err != nil {
		loadErr := fmt.Errorf("load source account: %w", err)
		logFailure("-", "load_source_account", loadErr.Error())
		return nil, loadErr
	}
	toAcc, err := s.repo.GetAccountSnapshot(ctx, req.ToAccountId)
	if err != nil {
		loadErr := fmt.Errorf("load destination account: %w", err)
		logFailure("-", "load_destination_account", loadErr.Error())
		return nil, loadErr
	}
	requestedAmount := money.FromDecimal(req.Amount, money.Currency(req.Currency))
	sourceBalance := money.FromDecimal(fromAcc.Balance, money.Currency(fromAcc.Currency))
	destinationBalance := money.FromDecimal(toAcc.Balance, money.Currency(toAcc.Currency))

	if !sourceBalance.SameCurrency(requestedAmount) || !destinationBalance.SameCurrency(requestedAmount) {
		logFailure("-", "validate_currency", ErrCurrencyMismatch.Error())
		return nil, ErrCurrencyMismatch
	}
	if fromAcc.Status != "ACTIVE" || toAcc.Status != "ACTIVE" {
		statusErr := fmt.Errorf("%w: accounts must be ACTIVE", ErrInvalidTransfer)
		logFailure("-", "validate_account_status", statusErr.Error())
		return nil, statusErr
	}

	insufficient, err := sourceBalance.LessThan(requestedAmount)
	if err != nil {
		logFailure("-", "balance_check", ErrCurrencyMismatch.Error())
		return nil, ErrCurrencyMismatch
	}
	if insufficient {
		logFailure("-", "balance_check", ErrInsufficientFunds.Error())
		return nil, ErrInsufficientFunds
	}

	clearingAcc, err := s.repo.GetAccountByCode(ctx, s.clearingCode)
	if errors.Is(err, ErrAccountNotFound) {
		logFailure("-", "load_clearing_account", ErrClearingAccountNotFound.Error())
		return nil, ErrClearingAccountNotFound
	}
	if err != nil {
		loadErr := fmt.Errorf("load clearing account: %w", err)
		logFailure("-", "load_clearing_account", loadErr.Error())
		return nil, loadErr
	}
	clearingBalance := money.FromDecimal(clearingAcc.Balance, money.Currency(clearingAcc.Currency))
	if !clearingBalance.SameCurrency(requestedAmount) {
		logFailure("-", "validate_clearing_currency", ErrCurrencyMismatch.Error())
		return nil, ErrCurrencyMismatch
	}
	if clearingAcc.Status != "ACTIVE" {
		statusErr := fmt.Errorf("%w: clearing account not ACTIVE", ErrInvalidTransfer)
		logFailure("-", "validate_clearing_status", statusErr.Error())
		return nil, statusErr
	}

	amountValue := requestedAmount.Amount()
	currencyCode := string(requestedAmount.Currency())
	tx := Transaction{
		IdempotencyKey: req.IdempotencyKey,
		FromAccountID:  req.FromAccountId,
		ToAccountID:    req.ToAccountId,
		Amount:         amountValue,
		Currency:       currencyCode,
		Status:         TxStatusPending,
		Description:    req.Description,
		Rail:           req.Rail,
	}

	created, err := s.repo.CreateTransaction(ctx, tx)
	if err != nil {
		if errors.Is(err, ErrDuplicateIdempotencyKey) {
			existing, getErr := s.repo.GetTransactionByIdempotencyKey(ctx, req.IdempotencyKey)
			if getErr == nil {
				return &existing, nil
			}
			if errors.Is(getErr, ErrTransactionNotFound) {
				logFailure("-", "load_duplicate_transaction", ErrTransferInProgress.Error())
				return nil, ErrTransferInProgress
			}
			dupErr := fmt.Errorf("lookup duplicate idempotency key: %w", getErr)
			logFailure("-", "load_duplicate_transaction", dupErr.Error())
			return nil, dupErr
		}
		createErr := fmt.Errorf("create transaction: %w", err)
		logFailure("-", "create_transaction", createErr.Error())
		return nil, createErr
	}
	transferID := created.ID.String()

	s.logger.Info().
		Str("event", "transfer_initiated").
		Str("trace_id", traceID).
		Str("transfer_id", transferID).
		Str("from_account", req.FromAccountId.String()).
		Str("to_account", req.ToAccountId.String()).
		Str("amount", amountValue.String()).
		Str("currency", currencyCode).
		Msg("transfer initiated")

	s.logger.Info().
		Str("event", "balance_checked").
		Str("trace_id", traceID).
		Str("transfer_id", transferID).
		Str("available_balance", sourceBalance.Amount().String()).
		Str("requested_amount", amountValue.String()).
		Msg("balance checked")

	// failTx transitions to FAILED, writes the reason, caches the outcome in the
	// idempotency store, and clears the in-progress lock so the defer doesn't
	// delete the final cached payload. It returns the original error unchanged so
	// callers can still propagate the right sentinel (e.g. ErrInsufficientFunds).
	failTx := func(step string, reason string, returnErr error) (*Transaction, error) {
		logFailure(transferID, step, reason)
		failed, ferr := s.repo.FailTransaction(ctx, created.ID, reason)
		if ferr != nil {
			return nil, fmt.Errorf("%w (additionally, fail-transition error: %v)", returnErr, ferr)
		}
		_ = s.idempotencyStoreResult(ctx, idemKey, failed)
		idemLocked = false
		return nil, returnErr
	}

	clearingEntry := ledger.JournalEntry{
		TransactionId: created.ID,
		Description:   "transfer clearing",
		Currency:      currencyCode,
		PostedBy:      s.postedBy,
		Lines: []ledger.JournalEntryLine{
			{AccountId: req.FromAccountId, Type: ledger.LineTypeDebit, Amount: amountValue, Sequence: 1},
			{AccountId: clearingAcc.ID, Type: ledger.LineTypeCredit, Amount: amountValue, Sequence: 2},
		},
	}
	clearingEntryID, err := s.ledger.PostJournalEntry(ctx, clearingEntry)
	if err != nil {
		if errors.Is(err, ledger.ErrInsufficientFunds) {
			return failTx("post_clearing_journal", "insufficient funds", ErrInsufficientFunds)
		}
		return failTx("post_clearing_journal", err.Error(), fmt.Errorf("post clearing entry: %w", err))
	}

	s.logger.Info().
		Str("event", "hold_placed").
		Str("trace_id", traceID).
		Str("transfer_id", transferID).
		Str("amount", amountValue.String()).
		Msg("hold-equivalent clearing posted")

	s.logger.Info().
		Str("event", "journal_entry_posted").
		Str("trace_id", traceID).
		Str("transfer_id", transferID).
		Str("journal_entry_id", clearingEntryID.String()).
		Str("debit_account", req.FromAccountId.String()).
		Str("credit_account", clearingAcc.ID.String()).
		Str("amount", amountValue.String()).
		Msg("journal entry posted")

	processing, err := s.repo.UpdateStatus(ctx, created.ID, TxStatusPending, TxStatusProcessing, nil)
	if err != nil {
		return failTx("transition_processing", err.Error(), fmt.Errorf("transition to PROCESSING: %w", err))
	}

	rail := InternalRail
	if req.Rail != nil && strings.TrimSpace(*req.Rail) != "" {
		rail = strings.ToUpper(strings.TrimSpace(*req.Rail))
	}
	if rail != InternalRail {
		if err := s.idempotencyStoreResult(ctx, idemKey, processing); err != nil {
			return nil, err
		}
		idemLocked = false
		return &processing, nil
	}

	settlementEntry := ledger.JournalEntry{
		TransactionId: created.ID,
		Description:   "transfer settlement",
		Currency:      currencyCode,
		PostedBy:      s.postedBy,
		Lines: []ledger.JournalEntryLine{
			{AccountId: clearingAcc.ID, Type: ledger.LineTypeDebit, Amount: amountValue, Sequence: 1},
			{AccountId: req.ToAccountId, Type: ledger.LineTypeCredit, Amount: amountValue, Sequence: 2},
		},
	}
	settlementEntryID, err := s.ledger.PostJournalEntry(ctx, settlementEntry)
	if err != nil {
		return failTx("post_settlement_journal", err.Error(), fmt.Errorf("post settlement entry: %w", err))
	}

	s.logger.Info().
		Str("event", "journal_entry_posted").
		Str("trace_id", traceID).
		Str("transfer_id", transferID).
		Str("journal_entry_id", settlementEntryID.String()).
		Str("debit_account", clearingAcc.ID.String()).
		Str("credit_account", req.ToAccountId.String()).
		Str("amount", amountValue.String()).
		Msg("journal entry posted")

	s.logger.Info().
		Str("event", "hold_released").
		Str("trace_id", traceID).
		Str("transfer_id", transferID).
		Str("amount", amountValue.String()).
		Msg("hold-equivalent clearing released")

	now := time.Now()
	settled, err := s.repo.UpdateStatus(ctx, created.ID, TxStatusProcessing, TxStatusSettled, &now)
	if err != nil {
		return failTx("transition_settled", err.Error(), fmt.Errorf("transition to SETTLED: %w", err))
	}
	if err := s.idempotencyStoreResult(ctx, idemKey, settled); err != nil {
		return nil, err
	}
	idemLocked = false

	s.logger.Info().
		Str("event", "transfer_settled").
		Str("trace_id", traceID).
		Str("transfer_id", transferID).
		Str("journal_entry_id", settlementEntryID.String()).
		Int64("duration_ms", time.Since(transferStart).Milliseconds()).
		Msg("transfer settled")

	return &settled, nil
}

func validateTransferRequest(req TransferRequest) error {
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return fmt.Errorf("%w: idempotency key is required", ErrInvalidTransfer)
	}
	if req.FromAccountId == uuid.Nil || req.ToAccountId == uuid.Nil {
		return fmt.Errorf("%w: account ids are required", ErrInvalidTransfer)
	}
	if req.FromAccountId == req.ToAccountId {
		return fmt.Errorf("%w: source and destination must differ", ErrInvalidTransfer)
	}
	normalizedCurrency := strings.ToUpper(strings.TrimSpace(req.Currency))
	transferAmount := money.FromDecimal(req.Amount, money.Currency(normalizedCurrency))
	if !transferAmount.IsPositive() {
		return fmt.Errorf("%w: amount must be > 0", ErrInvalidTransfer)
	}
	if normalizedCurrency == "" {
		return fmt.Errorf("%w: currency is required", ErrInvalidTransfer)
	}
	return nil
}

func encodeTransaction(tx Transaction) (string, error) {
	type idemTransactionPayload struct {
		ID             uuid.UUID  `json:"id"`
		IdempotencyKey string     `json:"idempotency_key"`
		FromAccountID  uuid.UUID  `json:"from_account_id"`
		ToAccountID    uuid.UUID  `json:"to_account_id"`
		Amount         string     `json:"amount"`
		Currency       string     `json:"currency"`
		Status         TxStatus   `json:"status"`
		Description    *string    `json:"description,omitempty"`
		Rail           *string    `json:"rail,omitempty"`
		ExternalRef    *string    `json:"external_ref,omitempty"`
		FailureReason  *string    `json:"failure_reason,omitempty"`
		JournalEntryID *uuid.UUID `json:"journal_entry_id,omitempty"`
		CreatedAt      time.Time  `json:"created_at"`
		UpdatedAt      time.Time  `json:"updated_at"`
		SettledAt      *time.Time `json:"settled_at,omitempty"`
		ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	}
	payload := idemTransactionPayload{
		ID:             tx.ID,
		IdempotencyKey: tx.IdempotencyKey,
		FromAccountID:  tx.FromAccountID,
		ToAccountID:    tx.ToAccountID,
		Amount:         tx.Amount.String(),
		Currency:       tx.Currency,
		Status:         tx.Status,
		Description:    tx.Description,
		Rail:           tx.Rail,
		ExternalRef:    tx.ExternalRef,
		FailureReason:  tx.FailureReason,
		JournalEntryID: tx.JournalEntryId,
		CreatedAt:      tx.CreatedAt,
		UpdatedAt:      tx.UpdatedAt,
		SettledAt:      tx.SettledAt,
		ExpiresAt:      tx.ExpiresAt,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeTransaction(raw string) (Transaction, error) {
	type idemTransactionPayload struct {
		ID             uuid.UUID  `json:"id"`
		IdempotencyKey string     `json:"idempotency_key"`
		FromAccountID  uuid.UUID  `json:"from_account_id"`
		ToAccountID    uuid.UUID  `json:"to_account_id"`
		Amount         string     `json:"amount"`
		Currency       string     `json:"currency"`
		Status         TxStatus   `json:"status"`
		Description    *string    `json:"description,omitempty"`
		Rail           *string    `json:"rail,omitempty"`
		ExternalRef    *string    `json:"external_ref,omitempty"`
		FailureReason  *string    `json:"failure_reason,omitempty"`
		JournalEntryID *uuid.UUID `json:"journal_entry_id,omitempty"`
		CreatedAt      time.Time  `json:"created_at"`
		UpdatedAt      time.Time  `json:"updated_at"`
		SettledAt      *time.Time `json:"settled_at,omitempty"`
		ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	}
	var payload idemTransactionPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return Transaction{}, err
	}
	amount, err := decimal.NewFromString(payload.Amount)
	if err != nil {
		return Transaction{}, err
	}
	return Transaction{
		ID:             payload.ID,
		IdempotencyKey: payload.IdempotencyKey,
		FromAccountID:  payload.FromAccountID,
		ToAccountID:    payload.ToAccountID,
		Amount:         amount,
		Currency:       payload.Currency,
		Status:         payload.Status,
		Description:    payload.Description,
		Rail:           payload.Rail,
		ExternalRef:    payload.ExternalRef,
		FailureReason:  payload.FailureReason,
		JournalEntryId: payload.JournalEntryID,
		CreatedAt:      payload.CreatedAt,
		UpdatedAt:      payload.UpdatedAt,
		SettledAt:      payload.SettledAt,
		ExpiresAt:      payload.ExpiresAt,
	}, nil
}
