package transaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/internal/ledger"
)

// TestTransfer_StressConcurrent20 fires 20 goroutines that all attempt to
// transfer GHS 100 from the same source account concurrently. Only as many
// should succeed as the starting balance allows (10 out of 20 with GHS 1000).
//
// Assertions:
//  - Total debited from Ernest == sum of all successful transfer amounts
//  - Sarfo received exactly what Ernest lost
//  - No money was created (Ernest_final + Sarfo_final == Ernest_initial)
//  - The ledger remains balanced (total debits == total credits)
func TestTransfer_StressConcurrent20(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping stress integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	suffix := uuid.New().String()[:8]
	ErnestID := uuid.New()
	SarfoID := uuid.New()
	clearingID := uuid.New()
	clearingCode := fmt.Sprintf("STRESS_CLEARING_%s", suffix)

	initialBalance := decimal.RequireFromString("1000.0000")
	transferAmount := decimal.RequireFromString("100.0000")

	for _, acc := range []struct {
		id      uuid.UUID
		code    string
		name    string
		balance string
	}{
		{ErnestID, fmt.Sprintf("STRESS_Ernest_%s", suffix), "Stress Ernest " + suffix, "1000.0000"},
		{SarfoID, fmt.Sprintf("STRESS_Sarfo_%s", suffix), "Stress Sarfo " + suffix, "0.0000"},
		{clearingID, clearingCode, "Stress Clearing " + suffix, "0.0000"},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO accounts (id, code, name, type, currency, balance, status, is_posting)
			VALUES ($1, $2, $3, 'LIABILITY', 'GHS', $4, 'ACTIVE', true)
		`, acc.id, acc.code, acc.name, acc.balance); err != nil {
			t.Fatalf("seed account %s: %v", acc.name, err)
		}
	}

	t.Cleanup(func() { cleanupStressAccounts(t, pool, ErnestID, SarfoID, clearingID) })

	ledgerRepo := ledger.NewLedgerRepository(pool)
	txRepo := NewPostgresRepository(pool)
	svc := NewTransferService(txRepo, ledger.NewLedger(ledgerRepo)).WithClearingCode(clearingCode)

	const workers = 20

	type result struct {
		ok bool
	}
	results := make([]result, workers)
	var wg sync.WaitGroup

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(n int) {
			defer wg.Done()
			_, err := svc.Transfer(ctx, TransferRequest{
				IdempotencyKey: fmt.Sprintf("stress-%s-%d", suffix, n),
				FromAccountId:  ErnestID,
				ToAccountId:    SarfoID,
				Amount:         transferAmount,
				Currency:       "GHS",
			})
			if err == nil {
				results[n] = result{ok: true}
				return
			}
			if !errors.Is(err, ErrInsufficientFunds) {
				t.Errorf("worker %d: unexpected error: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, r := range results {
		if r.ok {
			successCount++
		}
	}
	totalSucceeded := transferAmount.Mul(decimal.NewFromInt(int64(successCount)))

	// Query final balances from the DB (source of truth)
	var ErnestBalStr, SarfoBalStr string
	if err := pool.QueryRow(ctx, `SELECT balance::text FROM accounts WHERE id = $1`, ErnestID).Scan(&ErnestBalStr); err != nil {
		t.Fatalf("query Ernest balance: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT balance::text FROM accounts WHERE id = $1`, SarfoID).Scan(&SarfoBalStr); err != nil {
		t.Fatalf("query Sarfo balance: %v", err)
	}

	ErnestFinal, _ := decimal.NewFromString(ErnestBalStr)
	SarfoFinal, _ := decimal.NewFromString(SarfoBalStr)
	totalDebited := initialBalance.Sub(ErnestFinal)

	// total debited from Ernest must equal sum of successful transfer amounts
	if !totalDebited.Equal(totalSucceeded) {
		t.Errorf("total debited (%s) != sum of successful transfers (%s) — money lost or overdraft occurred",
			totalDebited, totalSucceeded)
	}

	// Sarfo received exactly what Ernest lost
	if !SarfoFinal.Equal(totalSucceeded) {
		t.Errorf("Sarfo final balance (%s) != total succeeded (%s) — settlement mismatch",
			SarfoFinal, totalSucceeded)
	}

	// Conservation: no money created or destroyed
	if !initialBalance.Equal(ErnestFinal.Add(SarfoFinal)) {
		t.Errorf("conservation violated: Ernest(%s) + Sarfo(%s) != initial(%s)",
			ErnestFinal, SarfoFinal, initialBalance)
	}

	// No transaction may be left in PENDING — every failure must have been
	// transitioned to FAILED so the system has a definitive, observable outcome.
	var pendingCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM transactions
		WHERE (from_account_id = $1 OR to_account_id = $1)
		  AND status = 'PENDING'
	`, ErnestID).Scan(&pendingCount); err != nil {
		t.Fatalf("query pending count: %v", err)
	}
	if pendingCount != 0 {
		t.Errorf("found %d orphaned PENDING transactions — failures must transition to FAILED", pendingCount)
	}

	// The number of SETTLED transactions must match the success count.
	var settledCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM transactions
		WHERE from_account_id = $1 AND status = 'SETTLED'
	`, ErnestID).Scan(&settledCount); err != nil {
		t.Fatalf("query settled count: %v", err)
	}
	if settledCount != successCount {
		t.Errorf("SETTLED count (%d) != success count (%d)", settledCount, successCount)
	}

	// Ledger-level double-entry invariant
	ledger.AssertLedgerBalanced(t, pool)

	t.Logf("stress test: %d/%d transfers succeeded | %d FAILED | 0 PENDING orphans | Ernest: %s → %s | Sarfo: 0 → %s",
		successCount, workers, workers-successCount, initialBalance, ErnestFinal, SarfoFinal)
}

// cleanupStressAccounts removes test data in FK-safe order.
func cleanupStressAccounts(t *testing.T, pool *pgxpool.Pool, ids ...uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	// Collect journal entry IDs that touched our accounts
	rows, _ := pool.Query(ctx,
		`SELECT DISTINCT entry_id FROM journal_entry_lines WHERE account_id = ANY($1)`, ids)
	var entryIDs []uuid.UUID
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if rows.Scan(&id) == nil {
				entryIDs = append(entryIDs, id)
			}
		}
	}

	// Collect transaction IDs
	txRows, _ := pool.Query(ctx,
		`SELECT id FROM transactions WHERE from_account_id = ANY($1) OR to_account_id = ANY($1)`, ids)
	var txIDs []uuid.UUID
	if txRows != nil {
		defer txRows.Close()
		for txRows.Next() {
			var id uuid.UUID
			if txRows.Scan(&id) == nil {
				txIDs = append(txIDs, id)
			}
		}
	}

	// Break circular FK: transactions.journal_entry_id ↔ journal_entries.transaction_id
	if len(txIDs) > 0 {
		pool.Exec(ctx, `UPDATE transactions SET journal_entry_id = NULL WHERE id = ANY($1)`, txIDs)
	}
	if len(entryIDs) > 0 {
		pool.Exec(ctx, `UPDATE journal_entries SET transaction_id = NULL WHERE id = ANY($1)`, entryIDs)
	}

	// Delete in dependency order
	pool.Exec(ctx, `DELETE FROM journal_entry_lines WHERE account_id = ANY($1)`, ids)
	if len(entryIDs) > 0 {
		pool.Exec(ctx, `DELETE FROM journal_entries WHERE id = ANY($1)`, entryIDs)
	}
	if len(txIDs) > 0 {
		pool.Exec(ctx, `DELETE FROM transactions WHERE id = ANY($1)`, txIDs)
	}
	pool.Exec(ctx, `DELETE FROM accounts WHERE id = ANY($1)`, ids)
}
