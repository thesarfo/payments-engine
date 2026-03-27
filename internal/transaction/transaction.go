package transaction

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/shopspring/decimal"
)

type TxStatus string

var ErrUnknownStatus = errors.New("unknown transaction status")

const (
	TxStatusPending    TxStatus = "PENDING"
	TxStatusProcessing TxStatus = "PROCESSING"
	TxStatusSettled    TxStatus = "SETTLED"
	TxStatusFailed     TxStatus = "FAILED"
	TxStatusReversed   TxStatus = "REVERSED"
	TxStatusOnHold     TxStatus = "ON_HOLD"
	TxStatusExpired    TxStatus = "EXPIRED"
)

type Transaction struct {
	ID             uuid.UUID
	IdempotencyKey string
	FromAccountID  uuid.UUID
	ToAccountID    uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	Status         TxStatus

	Description *string
	Metadata    pgtype.JSONB

	Rail          *string
	ExternalRef   *string
	FailureReason *string

	JournalEntryId *uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
	SettledAt *time.Time
	ExpiresAt *time.Time
}

var validTransitions = map[TxStatus][]TxStatus{
	TxStatusPending:    {TxStatusProcessing, TxStatusFailed, TxStatusExpired},
	TxStatusProcessing: {TxStatusSettled, TxStatusFailed},
	TxStatusSettled:    {TxStatusReversed},
	TxStatusFailed:     {},
	TxStatusReversed:   {},
}

func (tx *Transaction) TransitionTo(next TxStatus) error {
	if tx.Status == next {
		return fmt.Errorf("already in status %s", next)
	}
	allowed, ok := validTransitions[tx.Status]
	if !ok {
		return ErrUnknownStatus
	}

	for _, s := range allowed {
		if s == next {
			tx.Status = next
			tx.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("invalid transition: %s -> %s", tx.Status, next)
}
