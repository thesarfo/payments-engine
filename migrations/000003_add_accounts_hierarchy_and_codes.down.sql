DROP INDEX IF EXISTS idx_accounts_parent_id;
DROP INDEX IF EXISTS idx_accounts_code_unique;

ALTER TABLE accounts
  DROP COLUMN IF EXISTS is_posting,
  DROP COLUMN IF EXISTS parent_id,
  DROP COLUMN IF EXISTS code;

