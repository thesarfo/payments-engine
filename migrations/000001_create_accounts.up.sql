CREATE TYPE account_type AS ENUM (
    'ASSET',
    'LIABILITY',
    'EQUITY',
    'INCOME',
    'EXPENSE'
);

CREATE TYPE account_status AS ENUM (
    'ACTIVE',
    'FROZEN',
    'CLOSED'
);

CREATE TABLE accounts (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name      TEXT NOT NULL,
    type      account_type NOT NULL,
    currency  CHAR(3) NOT NULL,
    balance   NUMERIC(18, 4) NOT NULL DEFAULT 0,
    status    account_status NOT NULL DEFAULT 'ACTIVE',
    version   BIGINT NOT NULL DEFAULT 0
);
