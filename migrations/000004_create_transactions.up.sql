CREATE TYPE tx_status AS ENUM (
    'PENDING',
    'PROCESSING',
    'SETTLED',
    'FAILED',
    'REVERSED',
    'ON_HOLD',
    'EXPIRED'
);

CREATE TABLE transactions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key     TEXT UNIQUE NOT NULL,
    from_account_id     UUID NOT NULL REFERENCES accounts(id),
    to_account_id       UUID NOT NULL REFERENCES accounts(id),
    amount              NUMERIC(18, 4) NOT NULL CHECK (amount > 0),
    currency            CHAR(3) NOT NULL,
    status              tx_status NOT NULL DEFAULT 'PENDING',
    description         TEXT,
    metadata            JSONB,
    rail                TEXT,
    external_ref        TEXT,
    failure_reason      TEXT,
    journal_entry_id    UUID REFERENCES journal_entries(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    settled_at          TIMESTAMPTZ,
    expires_at          TIMESTAMPTZ,

    CHECK (from_account_id <> to_account_id)
);

ALTER TABLE journal_entries
  ADD CONSTRAINT fk_journal_entries_transaction_id
  FOREIGN KEY (transaction_id) REFERENCES transactions(id);

CREATE INDEX idx_transactions_from ON transactions(from_account_id);
CREATE INDEX idx_transactions_to ON transactions(to_account_id);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_idempotency ON transactions(idempotency_key);

