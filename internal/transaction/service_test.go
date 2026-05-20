package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/internal/audit"
	"github.com/thesarfo/payments-engine/internal/ledger"
	"github.com/thesarfo/payments-engine/pkg/idempotency"
)

type fakeRepo struct {
	accounts  map[uuid.UUID]AccountSnapshot
	byCode    map[string]AccountSnapshot
	tx        Transaction
	createErr error
	creates   int
	fails     int
}

func (f *fakeRepo) CreateTransaction(_ context.Context, tx Transaction) (Transaction, error) {
	if f.createErr != nil {
		return Transaction{}, f.createErr
	}
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

func (f *fakeRepo) GetTransactionByID(_ context.Context, txID uuid.UUID) (Transaction, error) {
	if f.tx.ID == uuid.Nil || f.tx.ID != txID {
		return Transaction{}, ErrTransactionNotFound
	}
	return f.tx, nil
}

func (f *fakeRepo) GetTransactionByIdempotencyKey(_ context.Context, idempotencyKey string) (Transaction, error) {
	if f.tx.ID == uuid.Nil || f.tx.IdempotencyKey != idempotencyKey {
		return Transaction{}, ErrTransactionNotFound
	}
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
	if a, ok := f.accounts[accountID]; ok {
		return a, nil
	}
	for _, v := range f.byCode {
		if v.ID == accountID {
			return v, nil
		}
	}
	return AccountSnapshot{}, ErrAccountNotFound
}

func (f *fakeRepo) GetAccountByCode(_ context.Context, code string) (AccountSnapshot, error) {
	a, ok := f.byCode[code]
	if !ok {
		return AccountSnapshot{}, ErrAccountNotFound
	}
	return a, nil
}

func (f *fakeRepo) ListSettledForSettlementDay(_ context.Context, _ time.Time, _ *time.Location, _ string) ([]Transaction, error) {
	return nil, nil
}

func (f *fakeRepo) BulkUpdateStatus(_ context.Context, _ []uuid.UUID, _, _ TxStatus) (int64, error) {
	return 0, nil
}

func (f *fakeRepo) UpdateStatusTx(_ context.Context, _ pgx.Tx, txID uuid.UUID, from TxStatus, to TxStatus, settledAt *time.Time) (Transaction, error) {
	return f.UpdateStatus(context.Background(), txID, from, to, settledAt)
}

// fakePool satisfies txBeginner for unit tests that don't need a real database.
type fakePool struct{}

func (f *fakePool) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return &fakeTx{}, nil
}

// fakeTx implements pgx.Tx. Only Commit and Rollback are needed; all other
// methods panic because the service fakes (fakeRepo, fakeLedgerRepo) ignore
// the tx argument.
type fakeTx struct{}

func (f *fakeTx) Begin(_ context.Context) (pgx.Tx, error)                      { return f, nil }
func (f *fakeTx) Commit(_ context.Context) error                                { return nil }
func (f *fakeTx) Rollback(_ context.Context) error                              { return nil }
func (f *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	panic("fakeTx.CopyFrom not implemented")
}
func (f *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults {
	panic("fakeTx.SendBatch not implemented")
}
func (f *fakeTx) LargeObjects() pgx.LargeObjects { panic("fakeTx.LargeObjects not implemented") }
func (f *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	panic("fakeTx.Prepare not implemented")
}
func (f *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	panic("fakeTx.Exec not implemented")
}
func (f *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	panic("fakeTx.Query not implemented")
}
func (f *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("fakeTx.QueryRow not implemented")
}
func (f *fakeTx) Conn() *pgx.Conn { panic("fakeTx.Conn not implemented") }

type fakeLedgerRepo struct {
	err     error
	calls   int
	entries []ledger.JournalEntry
}

func (f *fakeLedgerRepo) InsertJournalEntry(_ context.Context, entry ledger.JournalEntry) (uuid.UUID, error) {
	if f.err != nil {
		return uuid.Nil, f.err
	}
	f.calls++
	entry.ID = uuid.New()
	f.entries = append(f.entries, entry)
	return entry.ID, nil
}

func (f *fakeLedgerRepo) InsertJournalEntryTx(_ context.Context, _ pgx.Tx, entry ledger.JournalEntry) (uuid.UUID, error) {
	return f.InsertJournalEntry(context.Background(), entry)
}

type recordingAuditLogger struct {
	events []audit.AuditEvent
}

func (r *recordingAuditLogger) Log(_ context.Context, e audit.AuditEvent) error {
	p := make([]byte, len(e.Payload))
	copy(p, e.Payload)
	r.events = append(r.events, audit.AuditEvent{
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		EventType:  e.EventType,
		Actor:      e.Actor,
		Payload:    p,
	})
	return nil
}

func (r *recordingAuditLogger) GetByEntity(context.Context, string, uuid.UUID) ([]audit.AuditEvent, error) {
	return nil, nil
}

func (r *recordingAuditLogger) GetByEntityRange(context.Context, string, uuid.UUID, *time.Time, *time.Time) ([]audit.AuditEvent, error) {
	return nil, nil
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
	svc := NewTransferService(&fakePool{}, repo, ledger.NewLedger(ledgerRepo))

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
	svc := NewTransferService(&fakePool{}, repo, ledger.NewLedger(&fakeLedgerRepo{}))

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

func TestTransfer_CurrencyMismatch(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	clearingID := uuid.New()

	repo := &fakeRepo{
		accounts: map[uuid.UUID]AccountSnapshot{
			fromID: {ID: fromID, Currency: "GHS", Balance: decimal.RequireFromString("100.0000"), Status: "ACTIVE"},
			toID:   {ID: toID, Currency: "USD", Balance: decimal.RequireFromString("0.0000"), Status: "ACTIVE"},
		},
		byCode: map[string]AccountSnapshot{
			DefaultClearingCode: {ID: clearingID, Currency: "GHS", Balance: decimal.Zero, Status: "ACTIVE"},
		},
	}
	svc := NewTransferService(&fakePool{}, repo, ledger.NewLedger(&fakeLedgerRepo{}))

	_, err := svc.Transfer(context.Background(), TransferRequest{
		IdempotencyKey: "idem-currency-mismatch",
		FromAccountId:  fromID,
		ToAccountId:    toID,
		Amount:         decimal.RequireFromString("10.0000"),
		Currency:       "GHS",
	})
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
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
	svc := NewTransferService(&fakePool{}, repo, ledger.NewLedger(ledgerRepo))

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
	svc := NewTransferService(&fakePool{}, repo, ledger.NewLedger(ledgerRepo), idem)

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

func TestTransfer_DuplicateIdempotencyKeyWithoutStoreReturnsExisting(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	existingID := uuid.New()
	existing := Transaction{
		ID:             existingID,
		IdempotencyKey: "dup-db-key-1",
		FromAccountID:  fromID,
		ToAccountID:    toID,
		Amount:         decimal.RequireFromString("25.0000"),
		Currency:       "GHS",
		Status:         TxStatusSettled,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo := &fakeRepo{
		createErr: ErrDuplicateIdempotencyKey,
		tx:        existing,
		accounts: map[uuid.UUID]AccountSnapshot{
			fromID: {ID: fromID, Currency: "GHS", Balance: decimal.RequireFromString("100.0000"), Status: "ACTIVE"},
			toID:   {ID: toID, Currency: "GHS", Balance: decimal.RequireFromString("0.0000"), Status: "ACTIVE"},
		},
		byCode: map[string]AccountSnapshot{
			DefaultClearingCode: {ID: uuid.New(), Currency: "GHS", Balance: decimal.Zero, Status: "ACTIVE"},
		},
	}
	svc := NewTransferService(&fakePool{}, repo, ledger.NewLedger(&fakeLedgerRepo{}))

	got, err := svc.Transfer(context.Background(), TransferRequest{
		IdempotencyKey: "dup-db-key-1",
		FromAccountId:  fromID,
		ToAccountId:    toID,
		Amount:         decimal.RequireFromString("25.0000"),
		Currency:       "GHS",
	})
	if err != nil {
		t.Fatalf("Transfer() unexpected error: %v", err)
	}
	if got.ID != existingID {
		t.Fatalf("expected existing transaction id %s, got %s", existingID, got.ID)
	}
	if repo.creates != 0 {
		t.Fatalf("expected no successful creates, got %d", repo.creates)
	}
}

func TestGetTransactionByID(t *testing.T) {
	repo := &fakeRepo{
		tx: Transaction{
			ID:             uuid.New(),
			IdempotencyKey: "idem-get",
			Status:         TxStatusProcessing,
			Amount:         decimal.RequireFromString("10.0000"),
			Currency:       "GHS",
		},
	}
	svc := NewTransferService(&fakePool{}, repo, nil)

	got, err := svc.GetTransactionByID(context.Background(), repo.tx.ID)
	if err != nil {
		t.Fatalf("GetTransactionByID() unexpected error: %v", err)
	}
	if got.ID != repo.tx.ID {
		t.Fatalf("expected tx id %s, got %s", repo.tx.ID, got.ID)
	}
}

func TestGetTransactionByID_NotFound(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewTransferService(&fakePool{}, repo, nil)

	_, err := svc.GetTransactionByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestTransfer_AuditSnapshots_SettledInternalRail(t *testing.T) {
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
	al := &recordingAuditLogger{}
	svc := NewTransferService(&fakePool{}, repo, ledger.NewLedger(ledgerRepo)).WithAuditLogger(al)

	tx, err := svc.Transfer(context.Background(), TransferRequest{
		IdempotencyKey: "audit-idem-ok",
		FromAccountId:  fromID,
		ToAccountId:    toID,
		Amount:         decimal.RequireFromString("100.0000"),
		Currency:       "GHS",
	})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if len(al.events) != 15 {
		t.Fatalf("expected 15 audit events (5 steps x 3 fan-out), got %d", len(al.events))
	}
	want := []string{
		audit.EventTransferInitiated,
		audit.EventHoldPlaced,
		audit.EventJournalEntryPosted,
		audit.EventJournalEntryPosted,
		audit.EventTransferSettled,
	}
	for step := range want {
		base := step * 3
		triple := al.events[base : base+3]
		for j, e := range triple {
			if e.EventType != want[step] {
				t.Fatalf("event[%d]: want type %q, got %q", base+j, want[step], e.EventType)
			}
		}
		if triple[0].EntityType != audit.EntityTransaction || triple[0].EntityID != tx.ID {
			t.Fatalf("event[%d]: want transaction entity", base)
		}
		if triple[1].EntityType != audit.EntityAccount || triple[1].EntityID != fromID {
			t.Fatalf("event[%d]: want from account entity", base+1)
		}
		if triple[2].EntityType != audit.EntityAccount || triple[2].EntityID != toID {
			t.Fatalf("event[%d]: want to account entity", base+2)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(al.events[0].Payload, &payload); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if v, _ := payload["schema_version"].(float64); int(v) != audit.AuditPayloadSchemaVersion {
		t.Fatalf("schema_version: got %v", payload["schema_version"])
	}
	if payload["transaction"] == nil || payload["accounts"] == nil {
		t.Fatalf("expected transaction and accounts snapshots in payload, got %v", payload)
	}
	if payload["audit_scope"] != "transaction" {
		t.Fatalf("audit_scope: got %v", payload["audit_scope"])
	}
	var fromPayload map[string]any
	if err := json.Unmarshal(al.events[1].Payload, &fromPayload); err != nil {
		t.Fatalf("from payload: %v", err)
	}
	if fromPayload["audit_scope"] != "account" || fromPayload["account_role"] != "from" {
		t.Fatalf("from fan-out meta: %v", fromPayload)
	}
}

func TestTransfer_AuditSnapshots_FailsAfterInitiated(t *testing.T) {
	fromID := uuid.New()
	toID := uuid.New()
	clearingID := uuid.New()

	repo := &fakeRepo{
		accounts: map[uuid.UUID]AccountSnapshot{
			fromID: {ID: fromID, Currency: "GHS", Balance: decimal.RequireFromString("200.0000"), Status: "ACTIVE"},
			toID:   {ID: toID, Currency: "GHS", Balance: decimal.RequireFromString("0.0000"), Status: "ACTIVE"},
		},
		byCode: map[string]AccountSnapshot{
			DefaultClearingCode: {ID: clearingID, Currency: "GHS", Balance: decimal.Zero, Status: "ACTIVE"},
		},
	}
	ledgerRepo := &fakeLedgerRepo{err: ledger.ErrInsufficientFunds}
	al := &recordingAuditLogger{}
	svc := NewTransferService(&fakePool{}, repo, ledger.NewLedger(ledgerRepo)).WithAuditLogger(al)

	_, err := svc.Transfer(context.Background(), TransferRequest{
		IdempotencyKey: "audit-idem-fail",
		FromAccountId:  fromID,
		ToAccountId:    toID,
		Amount:         decimal.RequireFromString("200.0000"),
		Currency:       "GHS",
	})
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
	if len(al.events) != 6 {
		t.Fatalf("expected 6 audit events (2 steps x 3 fan-out), got %d", len(al.events))
	}
	for i, want := range []struct {
		typ        string
		entityType string
		id         uuid.UUID
	}{
		{audit.EventTransferInitiated, audit.EntityTransaction, repo.tx.ID},
		{audit.EventTransferInitiated, audit.EntityAccount, fromID},
		{audit.EventTransferInitiated, audit.EntityAccount, toID},
		{audit.EventTransferFailed, audit.EntityTransaction, repo.tx.ID},
		{audit.EventTransferFailed, audit.EntityAccount, fromID},
		{audit.EventTransferFailed, audit.EntityAccount, toID},
	} {
		if al.events[i].EventType != want.typ {
			t.Fatalf("event[%d] type: want %s, got %s", i, want.typ, al.events[i].EventType)
		}
		if al.events[i].EntityType != want.entityType {
			t.Fatalf("event[%d] entity: want %s, got %s", i, want.entityType, al.events[i].EntityType)
		}
		if al.events[i].EntityID != want.id {
			t.Fatalf("event[%d] entity id mismatch", i)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(al.events[3].Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload["failed_step"] != "post_clearing_journal" {
		t.Fatalf("failed_step: %v", payload["failed_step"])
	}
}
