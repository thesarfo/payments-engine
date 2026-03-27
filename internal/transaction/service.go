package transaction

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thesarfo/payments-engine/internal/ledger"
	"github.com/shopspring/decimal"
)

const (
	InternalRail       = "INTERNAL"
	DefaultClearingCode = "GL_LIAB_CLEARING"
	DefaultPostedBy     = "transfer-service"
)

var (
	ErrInvalidTransfer         = errors.New("invalid transfer request")
	ErrInsufficientFunds       = errors.New("insufficient funds")
	ErrCurrencyMismatch        = errors.New("currency mismatch")
	ErrClearingAccountNotFound = errors.New("clearing account not found")
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
}

func NewTransferService(repo Repository, ledgerSvc *ledger.Ledger) *TransferService {
	return &TransferService{
		repo:         repo,
		ledger:       ledgerSvc,
		clearingCode: DefaultClearingCode,
		postedBy:     DefaultPostedBy,
	}
}

func (s *TransferService) Transfer(ctx context.Context, req TransferRequest) (*Transaction, error) {
	if err := validateTransferRequest(req); err != nil {
		return nil, err
	}
	if s.ledger == nil {
		return nil, fmt.Errorf("%w: ledger service is required", ErrInvalidTransfer)
	}

	fromAcc, err := s.repo.GetAccountSnapshot(ctx, req.FromAccountId)
	if err != nil {
		return nil, fmt.Errorf("load source account: %w", err)
	}
	toAcc, err := s.repo.GetAccountSnapshot(ctx, req.ToAccountId)
	if err != nil {
		return nil, fmt.Errorf("load destination account: %w", err)
	}
	if fromAcc.Currency != req.Currency || toAcc.Currency != req.Currency {
		return nil, ErrCurrencyMismatch
	}
	if fromAcc.Status != "ACTIVE" || toAcc.Status != "ACTIVE" {
		return nil, fmt.Errorf("%w: accounts must be ACTIVE", ErrInvalidTransfer)
	}
	if fromAcc.Balance.LessThan(req.Amount) {
		return nil, ErrInsufficientFunds
	}

	clearingAcc, err := s.repo.GetAccountByCode(ctx, s.clearingCode)
	if errors.Is(err, ErrAccountNotFound) {
		return nil, ErrClearingAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load clearing account: %w", err)
	}
	if clearingAcc.Currency != req.Currency {
		return nil, ErrCurrencyMismatch
	}
	if clearingAcc.Status != "ACTIVE" {
		return nil, fmt.Errorf("%w: clearing account not ACTIVE", ErrInvalidTransfer)
	}

	tx := Transaction{
		IdempotencyKey: req.IdempotencyKey,
		FromAccountID:  req.FromAccountId,
		ToAccountID:    req.ToAccountId,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         TxStatusPending,
		Description:    req.Description,
		Rail:           req.Rail,
	}

	// 1. check if transaction already exists with same idempotency key in redis, in Pending state
	// 2. if it exists, return it. if not, proceed to create and store in postgres
	// 3. after creation, store the transaction in redis with a reasonable TTL (e.g., 5 minutes) to handle duplicates

	created, err := s.repo.CreateTransaction(ctx, tx)
	
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	clearingEntry := ledger.JournalEntry{
		TransactionId: created.ID,
		Description:   "transfer clearing",
		Currency:      req.Currency,
		PostedBy:      s.postedBy,
		Lines: []ledger.JournalEntryLine{
			{AccountId: req.FromAccountId, Type: ledger.LineTypeDebit, Amount: req.Amount, Sequence: 1},
			{AccountId: clearingAcc.ID, Type: ledger.LineTypeCredit, Amount: req.Amount, Sequence: 2},
		},
	}
	if err := s.ledger.PostJournalEntry(ctx, clearingEntry); err != nil {
		return nil, fmt.Errorf("post clearing entry: %w", err)
	}

	processing, err := s.repo.UpdateStatus(ctx, created.ID, TxStatusPending, TxStatusProcessing, nil)
	if err != nil {
		return nil, fmt.Errorf("transition to PROCESSING: %w", err)
	}

	rail := InternalRail
	if req.Rail != nil && strings.TrimSpace(*req.Rail) != "" {
		rail = strings.ToUpper(strings.TrimSpace(*req.Rail))
	}
	if rail != InternalRail {
		return &processing, nil
	}

	settlementEntry := ledger.JournalEntry{
		TransactionId: created.ID,
		Description:   "transfer settlement",
		Currency:      req.Currency,
		PostedBy:      s.postedBy,
		Lines: []ledger.JournalEntryLine{
			{AccountId: clearingAcc.ID, Type: ledger.LineTypeDebit, Amount: req.Amount, Sequence: 1},
			{AccountId: req.ToAccountId, Type: ledger.LineTypeCredit, Amount: req.Amount, Sequence: 2},
		},
	}
	if err := s.ledger.PostJournalEntry(ctx, settlementEntry); err != nil {
		return nil, fmt.Errorf("post settlement entry: %w", err)
	}

	now := time.Now()
	settled, err := s.repo.UpdateStatus(ctx, created.ID, TxStatusProcessing, TxStatusSettled, &now)
	if err != nil {
		return nil, fmt.Errorf("transition to SETTLED: %w", err)
	}
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
	if !req.Amount.GreaterThan(decimal.Zero) {
		return fmt.Errorf("%w: amount must be > 0", ErrInvalidTransfer)
	}
	if strings.TrimSpace(req.Currency) == "" {
		return fmt.Errorf("%w: currency is required", ErrInvalidTransfer)
	}
	return nil
}
