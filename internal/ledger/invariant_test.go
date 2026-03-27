package ledger

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAssertLedgerBalanced_CatchesDeliberatelyBrokenEntry(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping invariant integration test")
	}

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	suffix := uuid.New().String()[:8]
	accA := uuid.New()
	accB := uuid.New()

	_, err = tx.Exec(context.Background(), `
		INSERT INTO accounts (id, code, name, type, currency, status, is_posting)
		VALUES ($1, $2, $3, 'ASSET', 'GHS', 'ACTIVE', true)
	`, accA, "TEST_INV_A_"+suffix, "Invariant Test A "+suffix)
	if err != nil {
		t.Fatalf("insert account A: %v", err)
	}
	_, err = tx.Exec(context.Background(), `
		INSERT INTO accounts (id, code, name, type, currency, status, is_posting)
		VALUES ($1, $2, $3, 'LIABILITY', 'GHS', 'ACTIVE', true)
	`, accB, "TEST_INV_B_"+suffix, "Invariant Test B "+suffix)
	if err != nil {
		t.Fatalf("insert account B: %v", err)
	}

	entryID := uuid.New()
	_, err = tx.Exec(context.Background(), `
		INSERT INTO journal_entries (id, description, currency, status, posted_at, posted_by)
		VALUES ($1, $2, 'GHS', 'POSTED', $3, 'invariant-test')
	`, entryID, "Deliberately broken entry "+suffix, time.Now())
	if err != nil {
		t.Fatalf("insert journal entry: %v", err)
	}

	// intentionally unbalanced: debit 10, credit 9
	_, err = tx.Exec(context.Background(), `
		INSERT INTO journal_entry_lines (id, entry_id, account_id, type, amount, description, sequence)
		VALUES
			($1, $2, $3, 'DEBIT', 10.0000, 'broken debit', 1),
			($4, $2, $5, 'CREDIT', 9.0000, 'broken credit', 2)
	`, uuid.New(), entryID, accA, uuid.New(), accB)
	if err != nil {
		t.Fatalf("insert journal lines: %v", err)
	}

	err = checkLedgerBalanced(context.Background(), tx)
	if err == nil {
		t.Fatal("expected invariant check to fail for deliberately broken entry, got nil")
	}
	if err.Error() == "" {
		t.Fatalf("expected descriptive invariant error, got empty error")
	}
}
