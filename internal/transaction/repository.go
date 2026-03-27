package transaction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var (
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrInvalidStatusUpdate = errors.New("invalid transaction status transition")
	ErrAccountNotFound     = errors.New("account not found")
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

type AccountSnapshot struct {
	ID       uuid.UUID
	Code     string
	Name     string
	Type     string
	Currency string
	Balance  decimal.Decimal
	Status   string
	Version  int64
}

type Repository interface {
	CreateTransaction(ctx context.Context, tx Transaction) (Transaction, error)
	UpdateStatus(ctx context.Context, txID uuid.UUID, from TxStatus, to TxStatus, settledAt *time.Time) (Transaction, error)
	// FailTransaction transitions any PENDING or PROCESSING transaction to FAILED
	// and writes the reason. It is safe to call when the current status is already
	// FAILED (returns the existing row without error).
	FailTransaction(ctx context.Context, txID uuid.UUID, reason string) (Transaction, error)
	GetAccountSnapshot(ctx context.Context, accountID uuid.UUID) (AccountSnapshot, error)
	GetAccountByCode(ctx context.Context, code string) (AccountSnapshot, error)
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const insertTransactionSQL = `
INSERT INTO transactions (
	idempotency_key,
	from_account_id,
	to_account_id,
	amount,
	currency,
	status,
	description,
	metadata,
	rail,
	external_ref,
	failure_reason,
	journal_entry_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING
	id,
	idempotency_key,
	from_account_id,
	to_account_id,
	amount::text,
	currency,
	status,
	description,
	metadata,
	rail,
	external_ref,
	failure_reason,
	journal_entry_id,
	created_at,
	updated_at,
	settled_at,
	expires_at
`

const updateTransactionStatusSQL = `
UPDATE transactions
SET
	status = $3,
	updated_at = now(),
	settled_at = COALESCE($4, settled_at)
WHERE id = $1 AND status = $2
RETURNING
	id,
	idempotency_key,
	from_account_id,
	to_account_id,
	amount::text,
	currency,
	status,
	description,
	metadata,
	rail,
	external_ref,
	failure_reason,
	journal_entry_id,
	created_at,
	updated_at,
	settled_at,
	expires_at
`

const failTransactionSQL = `
UPDATE transactions
SET
	status        = 'FAILED',
	failure_reason = $2,
	updated_at    = now()
WHERE id = $1 AND status IN ('PENDING', 'PROCESSING')
RETURNING
	id,
	idempotency_key,
	from_account_id,
	to_account_id,
	amount::text,
	currency,
	status,
	description,
	metadata,
	rail,
	external_ref,
	failure_reason,
	journal_entry_id,
	created_at,
	updated_at,
	settled_at,
	expires_at
`

const selectTransactionByIDSQL = `
SELECT
	id,
	idempotency_key,
	from_account_id,
	to_account_id,
	amount::text,
	currency,
	status,
	description,
	metadata,
	rail,
	external_ref,
	failure_reason,
	journal_entry_id,
	created_at,
	updated_at,
	settled_at,
	expires_at
FROM transactions
WHERE id = $1
`

const selectAccountSnapshotByIDSQL = `
SELECT id, COALESCE(code, ''), name, type, currency, balance::text, status, version
FROM accounts
WHERE id = $1
`

const selectAccountSnapshotByCodeSQL = `
SELECT id, COALESCE(code, ''), name, type, currency, balance::text, status, version
FROM accounts
WHERE code = $1
`

func (r *PostgresRepository) CreateTransaction(ctx context.Context, tx Transaction) (Transaction, error) {
	row := r.pool.QueryRow(ctx, insertTransactionSQL,
		tx.IdempotencyKey,
		tx.FromAccountID,
		tx.ToAccountID,
		tx.Amount,
		tx.Currency,
		string(tx.Status),
		tx.Description,
		nullJSONB(tx.Metadata),
		tx.Rail,
		tx.ExternalRef,
		tx.FailureReason,
		tx.JournalEntryId,
	)
	out, err := scanTransactionRow(row)
	if err != nil {
		return Transaction{}, fmt.Errorf("create transaction: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) UpdateStatus(
	ctx context.Context,
	txID uuid.UUID,
	from TxStatus,
	to TxStatus,
	settledAt *time.Time,
) (Transaction, error) {
	row := r.pool.QueryRow(ctx, updateTransactionStatusSQL, txID, string(from), string(to), settledAt)
	out, err := scanTransactionRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Transaction{}, ErrInvalidStatusUpdate
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("update transaction status: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) FailTransaction(ctx context.Context, txID uuid.UUID, reason string) (Transaction, error) {
	row := r.pool.QueryRow(ctx, failTransactionSQL, txID, reason)
	out, err := scanTransactionRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
	
		row = r.pool.QueryRow(ctx, selectTransactionByIDSQL, txID)
		out, err = scanTransactionRow(row)
		if err != nil {
			return Transaction{}, fmt.Errorf("fail transaction (fetch terminal): %w", err)
		}
		return out, nil
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("fail transaction: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) GetAccountSnapshot(ctx context.Context, accountID uuid.UUID) (AccountSnapshot, error) {
	row := r.pool.QueryRow(ctx, selectAccountSnapshotByIDSQL, accountID)
	return scanAccountSnapshot(row)
}

func (r *PostgresRepository) GetAccountByCode(ctx context.Context, code string) (AccountSnapshot, error) {
	row := r.pool.QueryRow(ctx, selectAccountSnapshotByCodeSQL, code)
	return scanAccountSnapshot(row)
}

func scanAccountSnapshot(row pgx.Row) (AccountSnapshot, error) {
	var (
		id      uuid.UUID
		code    string
		name    string
		typeStr string
		curr    string
		balStr  string
		status  string
		version int64
	)
	if err := row.Scan(&id, &code, &name, &typeStr, &curr, &balStr, &status, &version); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AccountSnapshot{}, ErrAccountNotFound
		}
		return AccountSnapshot{}, err
	}
	balance, err := decimal.NewFromString(balStr)
	if err != nil {
		return AccountSnapshot{}, fmt.Errorf("parse account balance: %w", err)
	}
	return AccountSnapshot{
		ID:       id,
		Code:     code,
		Name:     name,
		Type:     typeStr,
		Currency: curr,
		Balance:  balance,
		Status:   status,
		Version:  version,
	}, nil
}

func scanTransactionRow(row pgx.Row) (Transaction, error) {
	var (
		t            Transaction
		amountStr    string
		statusStr    string
		description  pgtype.Text
		rail         pgtype.Text
		externalRef  pgtype.Text
		failure      pgtype.Text
		journalEntry pgtype.UUID
		settledAt    pgtype.Timestamptz
		expiresAt    pgtype.Timestamptz
	)
	if err := row.Scan(
		&t.ID,
		&t.IdempotencyKey,
		&t.FromAccountID,
		&t.ToAccountID,
		&amountStr,
		&t.Currency,
		&statusStr,
		&description,
		&t.Metadata,
		&rail,
		&externalRef,
		&failure,
		&journalEntry,
		&t.CreatedAt,
		&t.UpdatedAt,
		&settledAt,
		&expiresAt,
	); err != nil {
		return Transaction{}, err
	}

	amt, err := decimal.NewFromString(amountStr)
	if err != nil {
		return Transaction{}, fmt.Errorf("parse transaction amount: %w", err)
	}
	t.Amount = amt
	t.Status = TxStatus(statusStr)

	if description.Status == pgtype.Present {
		t.Description = &description.String
	}
	if rail.Status == pgtype.Present {
		t.Rail = &rail.String
	}
	if externalRef.Status == pgtype.Present {
		t.ExternalRef = &externalRef.String
	}
	if failure.Status == pgtype.Present {
		t.FailureReason = &failure.String
	}
	if journalEntry.Status == pgtype.Present {
		id, err := uuid.FromBytes(journalEntry.Bytes[:])
		if err != nil {
			return Transaction{}, fmt.Errorf("parse journal_entry_id: %w", err)
		}
		t.JournalEntryId = &id
	}
	if settledAt.Status == pgtype.Present {
		ts := settledAt.Time
		t.SettledAt = &ts
	}
	if expiresAt.Status == pgtype.Present {
		ts := expiresAt.Time
		t.ExpiresAt = &ts
	}
	return t, nil
}

func nullJSONB(v pgtype.JSONB) any {
	if v.Status == pgtype.Present {
		return v
	}
	return nil
}
