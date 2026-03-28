package ledger

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrUnbalancedEntry = errors.New("unbalanced journal entry: total debits must equal total credits")
var ErrEntriesQueryNotSupported = errors.New("entries query not supported by repository")

type Repository interface {
	InsertJournalEntry(ctx context.Context, entry JournalEntry) (uuid.UUID, error)
}

type accountEntriesReader interface {
	GetAccountEntryRows(ctx context.Context, accountID uuid.UUID) ([]AccountEntryRow, error)
}

type Ledger struct {
	repo Repository
}

func NewLedger(repo Repository) *Ledger {
	return &Ledger{repo: repo}
}

// PostJournalEntry enforces double-entry invariants and delegates to the repository
// to persist the entry atomically (header + lines + balance updates).
func (l *Ledger) PostJournalEntry(ctx context.Context, entry JournalEntry) (uuid.UUID, error) {
	totalDebits := decimal.Zero
	totalCredits := decimal.Zero

	for _, line := range entry.Lines {
		switch line.Type {
		case LineTypeDebit:
			totalDebits = totalDebits.Add(line.Amount)
		case LineTypeCredit:
			totalCredits = totalCredits.Add(line.Amount)
		default:
			return uuid.Nil, errors.New("invalid line type")
		}
	}

	if !totalDebits.Equal(totalCredits) {
		return uuid.Nil, ErrUnbalancedEntry
	}

	return l.repo.InsertJournalEntry(ctx, entry)
}

func (l *Ledger) GetAccountEntries(ctx context.Context, accountID uuid.UUID) ([]AccountEntryRow, error) {
	reader, ok := l.repo.(accountEntriesReader)
	if !ok {
		return nil, ErrEntriesQueryNotSupported
	}
	return reader.GetAccountEntryRows(ctx, accountID)
}
