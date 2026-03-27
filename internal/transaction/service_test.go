package transaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/internal/ledger"
	"github.com/thesarfo/payments-engine/pkg/idempotency"
)

type fakeRepo struct {
	accounts map[uuid.UUID]AccountSnapshot
	byCode   map[string]AccountSnapshot
	tx       Transaction
	creates  int
	fails    int
}

func (f *fakeRepo) CreateTransaction(_ context.Context, tx Transaction) (Transaction, error) {
	f.creates++
	tx.ID = uuid.New()
	tx.CreatedAt = time.Now()
	tx.UpdatedAt = tx.CreatedAt
	f.tx = tx
	return tx, nil
}

func (f *fakeRepo) FailTransaction(_ context.Context, _ uuid.UUID, reason string) (Transaction, error) {
	f.fails++
	f.tx.Status = TxStatusFailed
	f.tx.FailureReason = &reason
	f.tx.UpdatedAt = time.Now()
	return f.tx, nil
}

type fakeIdemStore struct {
	data map[string]string
}

func (f *fakeIdemStore) Get(_ context.Context, key string) (string, error) {
	v, ok := f.data[key]
	if !ok {
		return "", idempotency.ErrKeyNotFound
	}
	return v, nil
}

func (f *fakeIdemStore) Set(_ context.Context, key string, value string, _ time.Duration) error {
	f.data[key] = value
	return nil
}

func (f *fakeIdemStore) SetNX(_ context.Context, key string, value string, _ time.Duration) (bool, error) {
	if _, ok := f.data[key]; ok {
		return false, nil
	}
	f.data[key] = value
	return true, nil
}

func (f *fakeIdemStore) Del(_ context.Context, key string) error {
	delete(f.data, key)
	return nil
}

func (f *fakeRepo) UpdateStatus(_ context.Context, txID uuid.UUID, from TxStatus, to TxStatus, settledAt *time.Time) (Transaction, error) {
	if f.tx.ID != txID || f.tx.Status != from {
		return Transaction{}, ErrInvalidStatusUpdate
	}
	f.tx.Status = to
	f.tx.UpdatedAt = time.Now()
	if settledAt != nil {
		f.tx.SettledAt = settledAt
	}
	return f.tx, nil
}

func (f *fakeRepo) GetAccountSnapshot(_ context.Context, accountID uuid.UUID) (AccountSnapshot, error) {
	a, ok := f.accounts[accountID]
	if !ok {
		return AccountSnapshot{}, ErrAccountNotFound
	}
	return a, nil
}

func (f *fakeRepo) GetAccountByCode(_ context.Context, code string) (AccountSnapshot, error) {
	a, ok := f.byCode[code]
	if !ok {
		return AccountSnapshot{}, ErrAccountNotFound
	}
	return a, nil
}

type fakeLedgerRepo struct {
	err     error
	calls   int
	entries []ledger.JournalEntry
}

func (f *fakeLedgerRepo) InsertJournalEntry(_ context.Context, entry ledger.JournalEntry) error {
	if f.err != nil {
		return f.err
	}
	f.calls++
	f.entries = append(f.entries, entry)
	return nil
}

func TestTransfer_InternalSettlesAndPostsTwoEntries(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	clearingID := uuid.New()

	repo := &fakeRepo{
		accounts: map[uuid.UUID]AccountSnapshot{
			fromID: {ID: fromID, Currency: "GHS", Balance: decimal.RequireFromString("250.0000"), Status: "ACTIVE"},
			toID:   {ID: toID, Currency: "GHS", Balance: decimal.RequireFromString("10.0000"), Status: "ACTIVE"},
		},
		byCode: map[string]AccountSnapshot{
			DefaultClearingCode: {ID: clearingID, Currency: "GHS", Balance: decimal.Zero, Status: "ACTIVE"},
		},
	}
	ledgerRepo := &fakeLedgerRepo{}
	svc := NewTransferService(repo, ledger.NewLedger(ledgerRepo))

	tx, err := svc.Transfer(context.Background(), TransferRequest{
		IdempotencyKey: "idem-1",
		FromAccountId:  fromID,
		ToAccountId:    toID,
		Amount:         decimal.RequireFromString("100.0000"),
		Currency:       "GHS",
	})
	if err != nil {
		t.Fatalf("Transfer() unexpected error: %v", err)
	}

	if tx.Status != TxStatusSettled {
		t.Fatalf("expected SETTLED status, got %s", tx.Status)
	}
	if ledgerRepo.calls != 2 {
		t.Fatalf("expected 2 journal posts (clearing + settlement), got %d", ledgerRepo.calls)
	}
}

func TestTransfer_InsufficientFunds(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	clearingID := uuid.New()

	repo := &fakeRepo{
		accounts: map[uuid.UUID]AccountSnapshot{
			fromID: {ID: fromID, Currency: "GHS", Balance: decimal.RequireFromString("50.0000"), Status: "ACTIVE"},
			toID:   {ID: toID, Currency: "GHS", Balance: decimal.RequireFromString("0.0000"), Status: "ACTIVE"},
		},
		byCode: map[string]AccountSnapshot{
			DefaultClearingCode: {ID: clearingID, Currency: "GHS", Balance: decimal.Zero, Status: "ACTIVE"},
		},
	}
	svc := NewTransferService(repo, ledger.NewLedger(&fakeLedgerRepo{}))

	_, err := svc.Transfer(context.Background(), TransferRequest{
		IdempotencyKey: "idem-2",
		FromAccountId:  fromID,
		ToAccountId:    toID,
		Amount:         decimal.RequireFromString("100.0000"),
		Currency:       "GHS",
	})
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
	// The fast-path check catches this before a transaction is even created,
	// so there is nothing to transition to FAILED. Verify no create occurred.
	if repo.creates != 0 {
		t.Fatalf("expected no transaction created for fast-path insufficient-funds, got %d", repo.creates)
	}
}

// TestTransfer_RacePathInsufficientFunds simulates the window where the fast-path
// check passes (stale snapshot shows enough balance) but the ledger's post-lock
// check fires ErrInsufficientFunds. The transaction must land in FAILED state,
// not be left orphaned in PENDING.
func TestTransfer_RacePathInsufficientFunds(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	clearingID := uuid.New()

	repo := &fakeRepo{
		accounts: map[uuid.UUID]AccountSnapshot{
			// Snapshot shows 200 — enough for the fast-path check to pass.
			fromID: {ID: fromID, Currency: "GHS", Balance: decimal.RequireFromString("200.0000"), Status: "ACTIVE"},
			toID:   {ID: toID, Currency: "GHS", Balance: decimal.RequireFromString("0.0000"), Status: "ACTIVE"},
		},
		byCode: map[string]AccountSnapshot{
			DefaultClearingCode: {ID: clearingID, Currency: "GHS", Balance: decimal.Zero, Status: "ACTIVE"},
		},
	}

	// Ledger repo returns ErrInsufficientFunds, simulating the locked balance check.
	ledgerRepo := &fakeLedgerRepo{err: ledger.ErrInsufficientFunds}
	svc := NewTransferService(repo, ledger.NewLedger(ledgerRepo))

	_, err := svc.Transfer(context.Background(), TransferRequest{
		IdempotencyKey: "race-idem-1",
		FromAccountId:  fromID,
		ToAccountId:    toID,
		Amount:         decimal.RequireFromString("200.0000"),
		Currency:       "GHS",
	})
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
	// A PENDING transaction was created (fast-path passed), so it must be failed.
	if repo.creates != 1 {
		t.Fatalf("expected 1 transaction created, got %d", repo.creates)
	}
	if repo.fails != 1 {
		t.Fatalf("expected FailTransaction called once, got %d", repo.fails)
	}
	if repo.tx.Status != TxStatusFailed {
		t.Fatalf("expected transaction status FAILED, got %s", repo.tx.Status)
	}
}

func TestTransfer_DuplicateRequestReturnsOriginalTransaction(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	clearingID := uuid.New()

	repo := &fakeRepo{
		accounts: map[uuid.UUID]AccountSnapshot{
			fromID: {ID: fromID, Currency: "GHS", Balance: decimal.RequireFromString("500.0000"), Status: "ACTIVE"},
			toID:   {ID: toID, Currency: "GHS", Balance: decimal.RequireFromString("10.0000"), Status: "ACTIVE"},
		},
		byCode: map[string]AccountSnapshot{
			DefaultClearingCode: {ID: clearingID, Currency: "GHS", Balance: decimal.Zero, Status: "ACTIVE"},
		},
	}
	ledgerRepo := &fakeLedgerRepo{}
	idem := &fakeIdemStore{data: map[string]string{}}
	svc := NewTransferService(repo, ledger.NewLedger(ledgerRepo), idem)

	req := TransferRequest{
		IdempotencyKey: "dup-key-1",
		FromAccountId:  fromID,
		ToAccountId:    toID,
		Amount:         decimal.RequireFromString("100.0000"),
		Currency:       "GHS",
	}

	first, err := svc.Transfer(context.Background(), req)
	if err != nil {
		t.Fatalf("first Transfer() unexpected error: %v", err)
	}
	second, err := svc.Transfer(context.Background(), req)
	if err != nil {
		t.Fatalf("second Transfer() unexpected error: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected duplicate request to return same transaction id, got %s vs %s", first.ID, second.ID)
	}
	if repo.creates != 1 {
		t.Fatalf("expected one transaction creation, got %d", repo.creates)
	}
	if ledgerRepo.calls != 2 {
		t.Fatalf("expected one transfer posting (2 journal entries), got %d inserts", ledgerRepo.calls)
	}
}

