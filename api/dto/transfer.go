package dto

import (
	"encoding/json"
	"time"

	"github.com/thesarfo/payments-engine/internal/transaction"
)

type CreateTransferRequest struct {
	FromAccountID string  `json:"from_account_id" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	ToAccountID   string  `json:"to_account_id"   example:"b2c3d4e5-f6a7-8901-bcde-f12345678901"`
	Amount        string  `json:"amount"          example:"250.00"`
	Currency      string  `json:"currency"        example:"USD"`
	Rail          *string `json:"rail,omitempty"        example:"INTERNAL"`
	Description   *string `json:"description,omitempty" example:"Invoice #1042 payment"`
}

type TransactionResponse struct {
	ID             string  `json:"id"                       example:"d4e5f6a7-b8c9-0123-defa-234567890123"`
	IdempotencyKey string  `json:"idempotency_key"          example:"550e8400-e29b-41d4-a716-446655440000"`
	FromAccountID  string  `json:"from_account_id"          example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	ToAccountID    string  `json:"to_account_id"            example:"b2c3d4e5-f6a7-8901-bcde-f12345678901"`
	Amount         string  `json:"amount"                   example:"250.0000"`
	Currency       string  `json:"currency"                 example:"USD"`
	Status         string  `json:"status"                   example:"SETTLED" enums:"PENDING,PROCESSING,SETTLED,FAILED,REVERSED,ON_HOLD,EXPIRED,RECONCILED"`
	Description    *string `json:"description,omitempty"    example:"Invoice #1042 payment"`
	Rail           *string `json:"rail,omitempty"           example:"INTERNAL"`
	ExternalRef    *string `json:"external_ref,omitempty"   example:"ext-ref-001"`
	FailureReason  *string `json:"failure_reason,omitempty" example:"insufficient_funds"`
	JournalEntryID *string `json:"journal_entry_id,omitempty" example:"e5f6a7b8-c9d0-1234-efab-345678901234"`
	CreatedAt      string  `json:"created_at"               example:"2024-01-15T10:30:00.000000000Z"`
	UpdatedAt      string  `json:"updated_at"               example:"2024-01-15T10:30:01.000000000Z"`
	SettledAt      *string `json:"settled_at,omitempty"     example:"2024-01-15T10:30:01.000000000Z"`
	ExpiresAt      *string `json:"expires_at,omitempty"     example:"2024-01-16T10:30:00.000000000Z"`
}

func NewTransactionResponse(tx transaction.Transaction) TransactionResponse {
	txMoney := tx.Money()
	var (
		journalEntryID *string
		settledAt      *string
		expiresAt      *string
	)
	if tx.JournalEntryId != nil {
		v := tx.JournalEntryId.String()
		journalEntryID = &v
	}
	if tx.SettledAt != nil {
		v := tx.SettledAt.UTC().Format(time.RFC3339Nano)
		settledAt = &v
	}
	if tx.ExpiresAt != nil {
		v := tx.ExpiresAt.UTC().Format(time.RFC3339Nano)
		expiresAt = &v
	}
	return TransactionResponse{
		ID:             tx.ID.String(),
		IdempotencyKey: tx.IdempotencyKey,
		FromAccountID:  tx.FromAccountID.String(),
		ToAccountID:    tx.ToAccountID.String(),
		Amount:         txMoney.Amount().StringFixed(4),
		Currency:       tx.Currency,
		Status:         string(tx.Status),
		Description:    tx.Description,
		Rail:           tx.Rail,
		ExternalRef:    tx.ExternalRef,
		FailureReason:  tx.FailureReason,
		JournalEntryID: journalEntryID,
		CreatedAt:      tx.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:      tx.UpdatedAt.UTC().Format(time.RFC3339Nano),
		SettledAt:      settledAt,
		ExpiresAt:      expiresAt,
	}
}

type AuditEventResponse struct {
	ID         int64           `json:"id"          example:"1"`
	EntityType string          `json:"entity_type" example:"account"              enums:"account,transaction"`
	EntityID   string          `json:"entity_id"   example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	EventType  string          `json:"event_type"  example:"account.created"`
	Actor      string          `json:"actor"       example:"payments-engine"`
	Payload    json.RawMessage `json:"payload" swaggertype:"object"`
	OccurredAt string          `json:"occurred_at" example:"2024-01-15T10:30:00.000000000Z"`
}
