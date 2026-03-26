package ledger

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type EntryStatus string
type LineType string

const (
	EntryStatusPosted   EntryStatus = "POSTED"
	EntryStatusReversed EntryStatus = "REVERSED"
)

const (
	LineTypeDebit  LineType = "DEBIT"
	LineTypeCredit LineType = "CREDIT"
)

type JournalEntry struct {
	ID            uuid.UUID
	TransactionId uuid.UUID
	Reference     string
	Description   string
	Currency      string
	Status        EntryStatus
	Lines         []JournalEntryLine
	PostedAt      time.Time
	PostedBy      string
}

type JournalEntryLine struct {
	ID          uuid.UUID
	EntryId     uuid.UUID
	AccountId   uuid.UUID
	Type        LineType
	Amount      decimal.Decimal
	Description string
	Sequence    int16
}
