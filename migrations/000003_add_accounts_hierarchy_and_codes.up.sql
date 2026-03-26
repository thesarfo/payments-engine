ALTER TABLE accounts
  ADD COLUMN code TEXT,
  ADD COLUMN parent_id UUID REFERENCES accounts(id),
  ADD COLUMN is_posting BOOLEAN NOT NULL DEFAULT true;


WITH prepared AS (
  SELECT
    id,
    upper(regexp_replace(name, '[^A-Za-z0-9]+', '_', 'g')) AS base_code,
    row_number() OVER (
      PARTITION BY upper(regexp_replace(name, '[^A-Za-z0-9]+', '_', 'g'))
      ORDER BY id
    ) AS rn
  FROM accounts
  WHERE code IS NULL
)
UPDATE accounts a
SET code = CASE
  WHEN p.rn = 1 THEN p.base_code
  ELSE p.base_code || '_' || p.rn::text
END
FROM prepared p
WHERE a.id = p.id;

ALTER TABLE accounts
  ALTER COLUMN code SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_code_unique ON accounts(code);
CREATE INDEX IF NOT EXISTS idx_accounts_parent_id ON accounts(parent_id);

