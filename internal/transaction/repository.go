package transaction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var (
	ErrTransactionNotFound     = errors.New("transaction not found")
	ErrInvalidStatusUpdate     = errors.New("invalid transaction status transition")
	ErrAccountNotFound         = errors.New("account not found")
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
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
	UpdateStatusTx(ctx context.Context, dbtx pgx.Tx, txID uuid.UUID, from TxStatus, to TxStatus, settledAt *time.Time) (Transaction, error)
	FailTransaction(ctx context.Context, txID uuid.UUID, reason string) (Transaction, error)
	GetTransactionByID(ctx context.Context, txID uuid.UUID) (Transaction, error)
	GetTransactionByIdempotencyKey(ctx context.Context, idempotencyKey string) (Transaction, error)
	GetAccountSnapshot(ctx context.Context, accountID uuid.UUID) (AccountSnapshot, error)
	GetAccountByCode(ctx context.Context, code string) (AccountSnapshot, error)
	ListSettledForSettlementDay(ctx context.Context, day time.Time, loc *time.Location, currency string) ([]Transaction, error)
	BulkUpdateStatus(ctx context.Context, ids []uuid.UUID, from, to TxStatus) (int64, error)
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

const selectTransactionByIdempotencyKeySQL = `
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
WHERE idempotency_key = $1
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

const listSettledForSettlementDaySQL = `
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
WHERE status = 'SETTLED'
  AND settled_at IS NOT NULL
  AND settled_at >= $1
  AND settled_at < $2
  AND ($3::text = '' OR currency = $3)
ORDER BY settled_at, id
`

const bulkUpdateTransactionStatusSQL = `
UPDATE transactions
SET status = $3, updated_at = now()
WHERE id = ANY($1::uuid[]) AND status = $2
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Transaction{}, ErrDuplicateIdempotencyKey
		}
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

// UpdateStatusTx is identical to UpdateStatus but executes within a caller-supplied
// transaction. The caller owns commit/rollback.
func (r *PostgresRepository) UpdateStatusTx(
	ctx context.Context,
	dbtx pgx.Tx,
	txID uuid.UUID,
	from TxStatus,
	to TxStatus,
	settledAt *time.Time,
) (Transaction, error) {
	row := dbtx.QueryRow(ctx, updateTransactionStatusSQL, txID, string(from), string(to), settledAt)
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

func (r *PostgresRepository) GetTransactionByID(ctx context.Context, txID uuid.UUID) (Transaction, error) {
	row := r.pool.QueryRow(ctx, selectTransactionByIDSQL, txID)
	out, err := scanTransactionRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Transaction{}, ErrTransactionNotFound
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("get transaction by id: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) GetTransactionByIdempotencyKey(ctx context.Context, idempotencyKey string) (Transaction, error) {
	row := r.pool.QueryRow(ctx, selectTransactionByIdempotencyKeySQL, idempotencyKey)
	out, err := scanTransactionRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Transaction{}, ErrTransactionNotFound
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("get transaction by idempotency key: %w", err)
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


func SettlementDayBounds(day time.Time, loc *time.Location) (start, end time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	t := day.In(loc)
	start = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	end = start.Add(24 * time.Hour)
	return start, end
}

type pgxQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type pgxExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func (r *PostgresRepository) ListSettledForSettlementDay(ctx context.Context, day time.Time, loc *time.Location, currency string) ([]Transaction, error) {
	start, end := SettlementDayBounds(day, loc)
	return r.listSettledBetween(ctx, r.pool, start, end, currency)
}

func (r *PostgresRepository) ListSettledForSettlementDayTx(ctx context.Context, tx pgx.Tx, day time.Time, loc *time.Location, currency string) ([]Transaction, error) {
	start, end := SettlementDayBounds(day, loc)
	return r.listSettledBetween(ctx, tx, start, end, currency)
}

func (r *PostgresRepository) listSettledBetween(ctx context.Context, q pgxQuerier, start, end time.Time, currency string) ([]Transaction, error) {
	rows, err := q.Query(ctx, listSettledForSettlementDaySQL, start, end, currency)
	if err != nil {
		return nil, fmt.Errorf("list settled for settlement day: %w", err)
	}
	defer rows.Close()

	out := make([]Transaction, 0)
	for rows.Next() {
		t, err := scanTransactionRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list settled rows: %w", err)
	}
	return out, nil
}

func (r *PostgresRepository) BulkUpdateStatus(ctx context.Context, ids []uuid.UUID, from, to TxStatus) (int64, error) {
	return r.bulkUpdateStatus(ctx, r.pool, ids, from, to)
}

// BulkUpdateStatusTx updates status for the given ids within an existing transaction.
func (r *PostgresRepository) BulkUpdateStatusTx(ctx context.Context, tx pgx.Tx, ids []uuid.UUID, from, to TxStatus) (int64, error) {
	return r.bulkUpdateStatus(ctx, tx, ids, from, to)
}

func (r *PostgresRepository) bulkUpdateStatus(ctx context.Context, e pgxExecer, ids []uuid.UUID, from, to TxStatus) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := e.Exec(ctx, bulkUpdateTransactionStatusSQL, ids, string(from), string(to))
	if err != nil {
		return 0, fmt.Errorf("bulk update status: %w", err)
	}
	return tag.RowsAffected(), nil
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

func scanTransactionRow(row interface {
	Scan(dest ...any) error
}) (Transaction, error) {
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
