CREATE TABLE audit_events (
    id          BIGSERIAL PRIMARY KEY,             
    entity_type TEXT NOT NULL,                     
    entity_id   UUID NOT NULL,
    event_type  TEXT NOT NULL,                     
    actor       TEXT NOT NULL,                     
    payload     JSONB NOT NULL,                    
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_entity   ON audit_events(entity_type, entity_id);
CREATE INDEX idx_audit_occurred ON audit_events(occurred_at);

REVOKE UPDATE, DELETE ON audit_events FROM PUBLIC;
