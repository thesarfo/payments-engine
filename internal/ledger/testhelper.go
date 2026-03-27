package ledger

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// BalanceQuerier is the minimal interface required by AssertLedgerBalanced.
// Both *pgxpool.Pool and pgx.Tx satisfy it.
type BalanceQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// AssertLedgerBalanced fails the test if total debits != total credits across
// all journal_entry_lines. Call this after every mutation in integration tests
// to enforce the double-entry invariant.
func AssertLedgerBalanced(t *testing.T, db BalanceQuerier) {
	t.Helper()
	if err := checkLedgerBalanced(context.Background(), db); err != nil {
		t.Fatalf("ledger invariant violated: %v", err)
	}
}

func checkLedgerBalanced(ctx context.Context, db BalanceQuerier) error {
	var debitText, creditText string
	err := db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE type = 'DEBIT'), 0)::text  AS total_debits,
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
