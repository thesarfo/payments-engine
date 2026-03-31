package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thesarfo/payments-engine/api/dto"
	"github.com/thesarfo/payments-engine/internal/audit"
)

// parseAuditTimeQuery reads optional "from" and "to" query parameters as inclusive bounds on occurred_at
func parseAuditTimeQuery(r *http.Request) (from, to *time.Time, err error) {
	q := r.URL.Query()
	if s := strings.TrimSpace(q.Get("from")); s != "" {
		t, e := parseRFC3339Flexible(s)
		if e != nil {
			return nil, nil, fmt.Errorf("invalid from: expected RFC3339 timestamp")
		}
		from = &t
	}
	if s := strings.TrimSpace(q.Get("to")); s != "" {
		t, e := parseRFC3339Flexible(s)
		if e != nil {
			return nil, nil, fmt.Errorf("invalid to: expected RFC3339 timestamp")
		}
		to = &t
	}
	return from, to, nil
}

func parseRFC3339Flexible(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

func auditEventsToResponse(events []audit.AuditEvent) []dto.AuditEventResponse {
	out := make([]dto.AuditEventResponse, 0, len(events))
	for _, e := range events {
		out = append(out, dto.AuditEventResponse{
			ID:         e.ID,
			EntityType: e.EntityType,
			EntityID:   e.EntityID.String(),
			EventType:  e.EventType,
			Actor:      e.Actor,
			Payload:    json.RawMessage(e.Payload),
			OccurredAt: e.OccurredAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return out
}
