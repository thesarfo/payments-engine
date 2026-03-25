package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/thesarfo/payments-engine/api/handler"
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

	svc := account.NewService(repo)
	h := handler.NewAccountHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/accounts", h.CreateAccount)
	mux.HandleFunc("/api/v1/accounts/", h.GetAccountByID)

	addr := ":8080"
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}

}	