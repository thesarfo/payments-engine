package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"github.com/thesarfo/payments-engine/api/handler"
	apimiddleware "github.com/thesarfo/payments-engine/api/middleware"
	"github.com/thesarfo/payments-engine/internal/account"
	"github.com/thesarfo/payments-engine/internal/ledger"
	"github.com/thesarfo/payments-engine/internal/transaction"
	"github.com/thesarfo/payments-engine/pkg/idempotency"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("godotenv: %v (using existing env vars only)", err)
	}

	ctx := context.Background()
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		log.Fatal("database url is required")
	}

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		log.Fatalf("pgx pool: %v", err)
	}
	defer pool.Close()

	accountRepo := account.NewAccountRepository(pool)
	ledgerRepo := ledger.NewLedgerRepository(pool)
	transactionRepo := transaction.NewPostgresRepository(pool)

	svc := account.NewAccountService(accountRepo)
	ledgerSvc := ledger.NewLedger(ledgerRepo)
	accountHandler := handler.NewAccountHandler(svc, ledgerSvc)

	transferSvc := transaction.NewTransferService(transactionRepo, ledgerSvc)
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr != "" {
		redisClient := redis.NewClient(&redis.Options{
			Addr: redisAddr,
		})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("redis unavailable at %s, continuing without redis idempotency: %v", redisAddr, err)
			_ = redisClient.Close()
		} else {
			transferSvc = transaction.NewTransferService(
				transactionRepo,
				ledgerSvc,
				idempotency.NewRedisStore(redisClient),
			)
			defer func() {
				if err := redisClient.Close(); err != nil {
					log.Printf("redis close: %v", err)
				}
			}()
			log.Printf("redis idempotency enabled at %s", redisAddr)
		}
	}

	transferHandler := handler.NewTransferHandler(transferSvc)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(apimiddleware.RequestLogger)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/accounts", accountHandler.CreateAccount)
		r.Get("/accounts/{id}", accountHandler.GetAccountByID)
		r.Get("/accounts/{id}/entries", accountHandler.GetAccountEntries)
		r.Post("/transfers", transferHandler.CreateTransfer)
		r.Get("/transfers/{id}", transferHandler.GetTransferByID)
	})

	addr := ":8080"
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}

}
