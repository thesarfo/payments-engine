package transaction

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/thesarfo/payments-engine/internal/audit"
)

// transferAuditPayload is the JSON shape for audit_events.payload
// with full transaction and account snapshots after each state change
func transactionToSnapshotMap(tx Transaction) map[string]any {
	m := map[string]any{
		"id":              tx.ID.String(),
		"idempotency_key": tx.IdempotencyKey,
		"from_account_id": tx.FromAccountID.String(),
		"to_account_id":   tx.ToAccountID.String(),
		"amount":          tx.Amount.String(),
		"currency":        tx.Currency,
		"status":          string(tx.Status),
		"created_at":      tx.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      tx.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if tx.Description != nil {
		m["description"] = *tx.Description
	}
	if tx.Rail != nil {
		m["rail"] = *tx.Rail
	}
	if tx.ExternalRef != nil {
		m["external_ref"] = *tx.ExternalRef
	}
	if tx.FailureReason != nil {
		m["failure_reason"] = *tx.FailureReason
	}
	if tx.JournalEntryId != nil {
		m["journal_entry_id"] = tx.JournalEntryId.String()
	}
	if tx.SettledAt != nil {
		m["settled_at"] = tx.SettledAt.UTC().Format(time.RFC3339Nano)
	}
	if tx.ExpiresAt != nil {
		m["expires_at"] = tx.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	if meta := jsonMetadata(tx.Metadata); meta != nil {
		m["metadata"] = meta
	}
	return m
}

func jsonMetadata(m pgtype.JSONB) any {
	if m.Status != pgtype.Present || len(m.Bytes) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(m.Bytes, &v); err != nil {
		return string(m.Bytes)
	}
	return v
}

func accountSnapshotToMap(a AccountSnapshot) map[string]any {
	return map[string]any{
		"id":       a.ID.String(),
		"code":     a.Code,
		"name":     a.Name,
		"type":     a.Type,
		"currency": a.Currency,
		"balance":  a.Balance.String(),
		"status":   a.Status,
		"version":  a.Version,
	}
}

type journalAuditPart struct {
	EntryID     uuid.UUID
	Phase       string // clearing | settlement
	Description string
}

func (s *TransferService) buildTransferAuditPayload(
	traceID string,
	eventName string,
	tx Transaction,
	fromAcc, toAcc, clearingAcc AccountSnapshot,
	journal *journalAuditPart,
	failedStep, failureReason string,
	durationMs int64,
) map[string]any {
	out := map[string]any{
		"schema_version": audit.AuditPayloadSchemaVersion,
		"event":          eventName,
		"trace_id":       traceID,
		"transaction":    transactionToSnapshotMap(tx),
		"accounts": map[string]any{
			"from":     accountSnapshotToMap(fromAcc),
			"to":       accountSnapshotToMap(toAcc),
			"clearing": accountSnapshotToMap(clearingAcc),
		},
	}
	if journal != nil {
		out["journal"] = map[string]any{
			"entry_id":    journal.EntryID.String(),
			"phase":       journal.Phase,
			"description": journal.Description,
		}
	}
	if failedStep != "" {
		out["failed_step"] = failedStep
		out["failure_reason"] = failureReason
	}
	if durationMs > 0 {
		out["duration_ms"] = durationMs
	}
	return out
}

func (s *TransferService) loadThreeAccountSnapshots(ctx context.Context, fromID, toID, clearingID uuid.UUID) (fromAcc, toAcc, clearingAcc AccountSnapshot, err error) {
	fromAcc, err = s.repo.GetAccountSnapshot(ctx, fromID)
	if err != nil {
		return AccountSnapshot{}, AccountSnapshot{}, AccountSnapshot{}, fmt.Errorf("from account: %w", err)
	}
	toAcc, err = s.repo.GetAccountSnapshot(ctx, toID)
	if err != nil {
		return AccountSnapshot{}, AccountSnapshot{}, AccountSnapshot{}, fmt.Errorf("to account: %w", err)
	}
	clearingAcc, err = s.repo.GetAccountSnapshot(ctx, clearingID)
	if err != nil {
		return AccountSnapshot{}, AccountSnapshot{}, AccountSnapshot{}, fmt.Errorf("clearing account: %w", err)
	}
	return fromAcc, toAcc, clearingAcc, nil
}
