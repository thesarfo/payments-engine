package settlement

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/internal/transaction"
)


type NetPosition struct {
	FromAccountID    uuid.UUID
	ToAccountID      uuid.UUID
	Currency         string
	NetAmount        decimal.Decimal
	TransactionCount int
}

type NettingResult struct {
	Day             time.Time
	Location        *time.Location
	CurrencyFilter  string
	Positions       []NetPosition
	ReconciledCount int
}

// NetPositions aggregates transfers by (unordered account pair, currency), nets amounts, and returns
// only non-zero residual flows. Each position's TransactionCount is how many SETTLED transfers contributed to that pair.
func NetPositions(txs []transaction.Transaction) []NetPosition {
	type pairKey struct {
		low, high uuid.UUID
		currency  string
	}

	nets := make(map[pairKey]decimal.Decimal)
	counts := make(map[pairKey]int)

	for _, tx := range txs {
		low, high := orderAccounts(tx.FromAccountID, tx.ToAccountID)
		key := pairKey{low: low, high: high, currency: tx.Currency}
		var delta decimal.Decimal
		if tx.FromAccountID == low {
			delta = tx.Amount
		} else {
			delta = tx.Amount.Neg()
		}
		nets[key] = nets[key].Add(delta)
		counts[key]++
	}

	out := make([]NetPosition, 0, len(nets))
	for key, net := range nets {
		if net.IsZero() {
			continue
		}
		var from, to uuid.UUID
		var amount decimal.Decimal
		if net.IsPositive() {
			from, to = key.low, key.high
			amount = net
		} else {
			from, to = key.high, key.low
			amount = net.Abs()
		}
		out = append(out, NetPosition{
			FromAccountID:    from,
			ToAccountID:      to,
			Currency:         key.currency,
			NetAmount:        amount,
			TransactionCount: counts[key],
		})
	}

	sort.Slice(out, func(i, j int) bool { return positionLess(out[i], out[j]) })
	return out
}

func orderAccounts(a, b uuid.UUID) (low, high uuid.UUID) {
	if a.String() <= b.String() {
		return a, b
	}
	return b, a
}

func positionLess(a, b NetPosition) bool {
	fa, fb := a.FromAccountID.String(), b.FromAccountID.String()
	if fa != fb {
		return fa < fb
	}
	ta, tb := a.ToAccountID.String(), b.ToAccountID.String()
	if ta != tb {
		return ta < tb
	}
	return a.Currency < b.Currency
}
