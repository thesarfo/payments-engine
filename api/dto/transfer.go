package dto

import (
	"time"

	"github.com/thesarfo/payments-engine/internal/transaction"
)

type CreateTransferRequest struct {
	FromAccountID string  `json:"from_account_id"`
	ToAccountID   string  `json:"to_account_id"`
	Amount        string  `json:"amount"`
	Currency      string  `json:"currency"`
	Rail          *string `json:"rail,omitempty"`
	Description   *string `json:"description,omitempty"`
}

type TransactionResponse struct {
	ID             string  `json:"id"`
	IdempotencyKey string  `json:"idempotency_key"`
	FromAccountID  string  `json:"from_account_id"`
	ToAccountID    string  `json:"to_account_id"`
	Amount         string  `json:"amount"`
	Currency       string  `json:"currency"`
	Status         string  `json:"status"`
	Description    *string `json:"description,omitempty"`
	Rail           *string `json:"rail,omitempty"`
	ExternalRef    *string `json:"external_ref,omitempty"`
	FailureReason  *string `json:"failure_reason,omitempty"`
	JournalEntryID *string `json:"journal_entry_id,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	SettledAt      *string `json:"settled_at,omitempty"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
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
