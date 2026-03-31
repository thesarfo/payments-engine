package settlement

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thesarfo/payments-engine/internal/transaction"
)

// RunDailyNetting loads SETTLED transfers for the calendar day of day (in loc), nets them in memory,
// and atomically marks all included rows RECONCILED. currency empty means all currencies (netting still groups by currency).
// a second run for the same day finds no SETTLED rows and returns zero reconciled (idempotent).
func RunDailyNetting(
	ctx context.Context,
	pool *pgxpool.Pool,
	repo *transaction.PostgresRepository,
	day time.Time,
	loc *time.Location,
	currency string,
) (NettingResult, error) {
	dayStart, _ := transaction.SettlementDayBounds(day, loc)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return NettingResult{}, fmt.Errorf("begin settlement tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txs, err := repo.ListSettledForSettlementDayTx(ctx, tx, day, loc, currency)
	if err != nil {
		return NettingResult{}, err
	}

	res := NettingResult{
		Day:             dayStart,
		Location:        loc,
		CurrencyFilter:  currency,
		Positions:       NetPositions(txs),
		ReconciledCount: len(txs),
	}

	if len(txs) == 0 {
		if err := tx.Commit(ctx); err != nil {
			return NettingResult{}, fmt.Errorf("commit empty settlement: %w", err)
		}
		return res, nil
	}

	ids := make([]uuid.UUID, len(txs))
	for i := range txs {
		ids[i] = txs[i].ID
	}

	n, err := repo.BulkUpdateStatusTx(ctx, tx, ids, transaction.TxStatusSettled, transaction.TxStatusReconciled)
	if err != nil {
		return NettingResult{}, err
	}
	if n != int64(len(txs)) {
		return NettingResult{}, fmt.Errorf("expected %d rows reconciled, updated %d", len(txs), n)
	}

	if err := tx.Commit(ctx); err != nil {
		return NettingResult{}, fmt.Errorf("commit settlement: %w", err)
	}
	return res, nil
}
