package transaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/internal/ledger"
)

type fakeRepo struct {
	accounts map[uuid.UUID]AccountSnapshot
	byCode   map[string]AccountSnapshot
	tx       Transaction
}

func (f *fakeRepo) CreateTransaction(_ context.Context, tx Transaction) (Transaction, error) {
	tx.ID = uuid.New()
	tx.CreatedAt = time.Now()
	tx.UpdatedAt = tx.CreatedAt
	f.tx = tx
	return tx, nil
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
	calls   int
	entries []ledger.JournalEntry
}

func (f *fakeLedgerRepo) InsertJournalEntry(_ context.Context, entry ledger.JournalEntry) error {
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
}

