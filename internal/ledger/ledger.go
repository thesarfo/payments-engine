package ledger

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
)

// ErrUnbalancedEntry is returned when total debits != total credits.
var ErrUnbalancedEntry = errors.New("unbalanced journal entry: total debits must equal total credits")

// Repository is the persistence port for the ledger engine.
type Repository interface {
	InsertJournalEntry(ctx context.Context, entry JournalEntry) error
}

type Ledger struct {
	repo Repository
}

func NewLedger(repo Repository) *Ledger {
	return &Ledger{repo: repo}
}

// PostJournalEntry enforces double-entry invariants and delegates to the repository
// to persist the entry atomically (header + lines + balance updates).
func (l *Ledger) PostJournalEntry(ctx context.Context, entry JournalEntry) error {
	// 1) Debits == credits (amounts are positive; type determines side).
	totalDebits := decimal.Zero
	totalCredits := decimal.Zero

	for _, line := range entry.Lines {
		switch line.Type {
		case LineTypeDebit:
			totalDebits = totalDebits.Add(line.Amount)
		case LineTypeCredit:
			totalCredits = totalCredits.Add(line.Amount)
		default:
			return errors.New("invalid line type")
		}
	}

	if !totalDebits.Equal(totalCredits) {
		return ErrUnbalancedEntry
	}

	// 2) + 3) The repository performs: accounts exist + ACTIVE checks,
	// and transactionally inserts entry/lines and updates balances.
	return l.repo.InsertJournalEntry(ctx, entry)
}
