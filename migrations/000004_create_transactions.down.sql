ALTER TABLE journal_entries
  DROP CONSTRAINT IF EXISTS fk_journal_entries_transaction_id;

DROP INDEX IF EXISTS idx_transactions_idempotency;
DROP INDEX IF EXISTS idx_transactions_status;
DROP INDEX IF EXISTS idx_transactions_to;
DROP INDEX IF EXISTS idx_transactions_from;

DROP TABLE IF EXISTS transactions;
DROP TYPE IF EXISTS tx_status;

