-- Ensure pgcrypto exists for gen_random_uuid() (required by our schema migrations).
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE entry_status AS ENUM ('POSTED', 'REVERSED');

CREATE TYPE line_type AS ENUM ('DEBIT', 'CREDIT');

CREATE TABLE journal_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id  UUID,                          -- links to transactions table (nullable for now)
    reference       TEXT,                         
    description     TEXT NOT NULL,
    currency        CHAR(3) NOT NULL,
    status          entry_status NOT NULL DEFAULT 'POSTED',
    posted_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    posted_by       TEXT NOT NULL                
);

CREATE TABLE journal_entry_lines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id        UUID NOT NULL REFERENCES journal_entries(id),
    account_id      UUID NOT NULL REFERENCES accounts(id),
    type            line_type NOT NULL,
    amount          NUMERIC(18, 4) NOT NULL CHECK (amount > 0),
    description     TEXT,
    sequence        SMALLINT NOT NULL DEFAULT 0    -- ordering within entry
);

CREATE INDEX idx_entry_lines_account ON journal_entry_lines(account_id);
CREATE INDEX idx_entry_lines_entry ON journal_entry_lines(entry_id);
CREATE INDEX idx_journal_entries_tx ON journal_entries(transaction_id);

