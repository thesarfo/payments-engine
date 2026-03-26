package ledger

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type balanceQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// AssertLedgerBalanced: total debits must equal total credits.
func AssertLedgerBalanced(t *testing.T, db balanceQuerier) {
	t.Helper()
	if err := checkLedgerBalanced(context.Background(), db); err != nil {
		t.Fatalf("ledger invariant violated: %v", err)
	}
}

func checkLedgerBalanced(ctx context.Context, db balanceQuerier) error {
	var debitText, creditText string
	err := db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE type = 'DEBIT'), 0)::text AS total_debits,
			COALESCE(SUM(amount) FILTER (WHERE type = 'CREDIT'), 0)::text AS total_credits
		FROM journal_entry_lines
	`).Scan(&debitText, &creditText)
	if err != nil {
		return fmt.Errorf("query totals: %w", err)
	}

	totalDebits, err := decimal.NewFromString(debitText)
	if err != nil {
		return fmt.Errorf("parse total debits: %w", err)
	}
	totalCredits, err := decimal.NewFromString(creditText)
	if err != nil {
		return fmt.Errorf("parse total credits: %w", err)
	}

	if !totalDebits.Equal(totalCredits) {
		return fmt.Errorf("total debits %s != total credits %s", totalDebits, totalCredits)
	}
	return nil
}

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
