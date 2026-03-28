package main

import (
	"context"
	"errors"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/thesarfo/payments-engine/config"
)

type seedAccount struct {
	Code       string
	Name       string
	Type       string
	Currency   string
	IsPosting  bool
	ParentCode string
}

var baseCoA = []seedAccount{
	// Roots (control accounts)
	{Code: "GL_ASSETS", Name: "Assets", Type: "ASSET", Currency: "GHS", IsPosting: false},
	{Code: "GL_LIABILITIES", Name: "Liabilities", Type: "LIABILITY", Currency: "GHS", IsPosting: false},
	{Code: "GL_INCOME", Name: "Income", Type: "INCOME", Currency: "GHS", IsPosting: false},

	// Children (posting / control)
	{Code: "GL_ASSET_VAULT_CASH", Name: "Vault Cash", Type: "ASSET", Currency: "GHS", IsPosting: true, ParentCode: "GL_ASSETS"},
	{Code: "GL_ASSET_SETTLEMENT", Name: "Settlement Account", Type: "ASSET", Currency: "GHS", IsPosting: true, ParentCode: "GL_ASSETS"},

	// Pool/control concept (not posting; customers later sit under it)
	{Code: "GL_LIAB_CUSTOMER_SAVINGS_POOL", Name: "Customer Savings Pool", Type: "LIABILITY", Currency: "GHS", IsPosting: false, ParentCode: "GL_LIABILITIES"},
	{Code: "GL_LIAB_CLEARING", Name: "Clearing Account", Type: "LIABILITY", Currency: "GHS", IsPosting: true, ParentCode: "GL_LIABILITIES"},

	{Code: "GL_INCOME_FEES", Name: "Fee Income", Type: "INCOME", Currency: "GHS", IsPosting: true, ParentCode: "GL_INCOME"},
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("godotenv: %v (using existing env vars only)", err)
	}

	cfg, err := config.LoadSeedConfig()
	if err != nil {
		log.Fatalf("load seed config: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("pgx pool: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inserted := 0

	for _, a := range baseCoA {
		if a.ParentCode != "" {
			continue
		}
		didInsert, err := ensureAccount(ctx, tx, a, uuid.Nil)
		if err != nil {
			log.Fatalf("seed root %s: %v", a.Code, err)
		}
		if didInsert {
			inserted++
		}
	}

	for _, a := range baseCoA {
		if a.ParentCode == "" {
			continue
		}
		parentID, err := getAccountIDByCode(ctx, tx, a.ParentCode)
		if err != nil {
			log.Fatalf("load parent %s for %s: %v", a.ParentCode, a.Code, err)
		}
		didInsert, err := ensureAccount(ctx, tx, a, parentID)
		if err != nil {
			log.Fatalf("seed child %s: %v", a.Code, err)
		}
		if didInsert {
			inserted++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit: %v", err)
	}

	log.Printf("seed complete: inserted=%d total=%d", inserted, len(baseCoA))
}

func ensureAccount(ctx context.Context, tx pgx.Tx, a seedAccount, parentID uuid.UUID) (bool, error) {
	exists, err := accountExistsByCode(ctx, tx, a.Code)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	if err := insertAccount(ctx, tx, a, parentID); err != nil {
		return false, err
	}
	return true, nil
}

func accountExistsByCode(ctx context.Context, tx pgx.Tx, code string) (bool, error) {
	var one int
	err := tx.QueryRow(ctx, `
		SELECT 1
		FROM accounts
		WHERE code = $1
		LIMIT 1
	`, code).Scan(&one)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func getAccountIDByCode(ctx context.Context, tx pgx.Tx, code string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM accounts
		WHERE code = $1
	`, code).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func insertAccount(ctx context.Context, tx pgx.Tx, a seedAccount, parentID uuid.UUID) error {
	var parent any = nil
	if parentID != uuid.Nil {
		parent = parentID
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO accounts (code, name, type, currency, status, parent_id, is_posting)
		VALUES ($1, $2, $3, $4, 'ACTIVE', $5, $6)
	`, a.Code, a.Name, a.Type, a.Currency, parent, a.IsPosting)
	return err
}
