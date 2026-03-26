package account

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/pkg/money"
)

var ErrAccountNotFound = errors.New("account not found")

type Repository interface {
	CreateAccount(ctx context.Context, a Account) (Account, error)
	GetAccountByID(ctx context.Context, id uuid.UUID) (Account, error)
}

type AccountRepository struct {
	pool *pgxpool.Pool
}

func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

const insertAccountSQL = `
INSERT INTO accounts (code, name, "type", currency, balance, status, is_posting)
VALUES ($1, $2, $3, $4, $5, $6, true)
RETURNING id, name, "type", currency, balance, status, version
`

const selectAccountByIDSQL = `
SELECT id, name, "type", currency, balance, status, version
FROM accounts
WHERE id = $1
`

func (r *AccountRepository) CreateAccount(ctx context.Context, a Account) (Account, error) {
	if a.Name == "" {
		return Account{}, fmt.Errorf("account name is required")
	}

	status := a.Status
	if status == "" {
		status = AccountStatusActive
	}

	row := r.pool.QueryRow(ctx, insertAccountSQL,
		generateAccountCode(a.Name),
		a.Name,
		string(a.Type),
		string(a.Currency),
		a.Balance,
		string(status),
	)

	out, err := scanAccount(row)
	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}
	return out, nil
}

func generateAccountCode(name string) string {
	base := strings.ToUpper(strings.TrimSpace(name))
	base = regexp.MustCompile(`[^A-Z0-9]+`).ReplaceAllString(base, "_")
	base = strings.Trim(base, "_")
	if base == "" {
		base = "ACCOUNT"
	}
	return fmt.Sprintf("%s_%s", base, strings.ToUpper(uuid.NewString()[:8]))
}

func (r *AccountRepository) GetAccountByID(ctx context.Context, id uuid.UUID) (Account, error) {
	row := r.pool.QueryRow(ctx, selectAccountByIDSQL, id)
	out, err := scanAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("get account: %w", err)
	}
	return out, nil
}

func scanAccount(row pgx.Row) (Account, error) {
	var (
		dbUUID  pgtype.UUID
		name    string
		typeStr string
		curr    string
		balStr  string
		statStr string
		version int64
	)
	if err := row.Scan(&dbUUID, &name, &typeStr, &curr, &balStr, &statStr, &version); err != nil {
		return Account{}, err
	}
	if !dbUUID.Valid {
		return Account{}, fmt.Errorf("account id is null")
	}
	id, err := uuid.FromBytes(dbUUID.Bytes[:])
	if err != nil {
		return Account{}, err
	}
	balance, err := decimal.NewFromString(balStr)
	if err != nil {
		return Account{}, fmt.Errorf("parse balance: %w", err)
	}
	return Account{
		ID:       id,
		Name:     name,
		Type:     AccountType(typeStr),
		Currency: money.Currency(curr),
		Balance:  balance,
		Status:   AccountStatus(statStr),
		Version:  version,
	}, nil
}
