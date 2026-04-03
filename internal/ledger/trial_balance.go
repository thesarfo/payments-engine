package ledger

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// TrialBalance is the aggregated trial balance for one currency.
type TrialBalance struct {
	Currency string
	Rows     []TrialBalanceLine
	Balanced bool
	NetTotal decimal.Decimal // sum of row nets; should be zero when the ledger is consistent
}

// TrialBalanceLine is one account row in a trial balance (debits and credits from journal lines).
type TrialBalanceLine struct {
	AccountID    uuid.UUID
	Code         string
	Name         string
	TotalDebits  decimal.Decimal
	TotalCredits decimal.Decimal
	Net          decimal.Decimal // total_debits - total_credits
}
