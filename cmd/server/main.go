package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/thesarfo/payments-engine/internal/account"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("godotenv: %v (using existing env vars only)", err)
	}

	ctx := context.Background()
	connString := os.Getenv("DATABASE_URL")
	if connString == ""{
		log.Fatal("database url is required")
	}

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		log.Fatalf("pgx pool: %v", err)
	}
	defer pool.Close()

	repo := account.NewPostgresAccountRepository(pool)

	created, err := repo.CreateAccount(ctx, account.Account{
		Name:     "Cash",
		Type:     account.AccountTypeAsset,
		Currency: "USD",
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("created %+v", created)

	loaded, err := repo.GetAccountByID(ctx, created.ID)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded %+v", loaded)


}	