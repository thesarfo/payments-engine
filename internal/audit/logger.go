package audit

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)


type Logger interface {
	Log(ctx context.Context, event AuditEvent) error
	GetByEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]AuditEvent, error)
}

type PostgresLogger struct {
	pool *pgxpool.Pool
}

func NewPostgresLogger(pool *pgxpool.Pool) *PostgresLogger {
	return &PostgresLogger{pool: pool}
}

const insertAuditEventSQL = `
INSERT INTO audit_events (entity_type, entity_id, event_type, actor, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, occurred_at`

// appends an audit event. Returns an error only if the write fails
// callers should treat this as best-effort and not fail the primary operation
func (l *PostgresLogger) Log(ctx context.Context, event AuditEvent) error {
	payload := pgtype.Text{String: string(event.Payload), Valid: true}
	return l.pool.QueryRow(
		ctx,
		insertAuditEventSQL,
		event.EntityType,
		event.EntityID,
		event.EventType,
		event.Actor,
		payload,
	).Scan(&event.ID, &event.OccurredAt)
}

const selectAuditEventsSQL = `
SELECT id, entity_type, entity_id, event_type, actor, payload, occurred_at
FROM   audit_events
WHERE  entity_type = $1 AND entity_id = $2
ORDER  BY id ASC`

// GetByEntity returns all audit events for a given entity in order
func (l *PostgresLogger) GetByEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]AuditEvent, error) {
	rows, err := l.pool.Query(ctx, selectAuditEventsSQL, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(
			&e.ID,
			&e.EntityType,
			&e.EntityID,
			&e.EventType,
			&e.Actor,
			&e.Payload,
			&e.OccurredAt,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}