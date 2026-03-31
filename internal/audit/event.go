package audit

import (
	"time"

	"github.com/google/uuid"
)

const (
	EventAccountCreated    = "ACCOUNT_CREATED"
	EventTransferInitiated = "TRANSFER_INITIATED"
	EventTransferSettled   = "TRANSFER_SETTLED"
	EventTransferFailed    = "TRANSFER_FAILED"
)

const (
	EntityAccount     = "account"
	EntityTransaction = "transaction"
)


type AuditEvent struct {
	ID         int64
	EntityType string
	EntityID   uuid.UUID
	EventType  string
	Actor      string
	Payload    []byte
	OccurredAt time.Time
}