package main

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"github.com/thesarfo/payments-engine/api/handler"
	apimiddleware "github.com/thesarfo/payments-engine/api/middleware"
	"github.com/thesarfo/payments-engine/config"
	"github.com/thesarfo/payments-engine/internal/account"
	"github.com/thesarfo/payments-engine/internal/ledger"
	"github.com/thesarfo/payments-engine/internal/transaction"
	"github.com/thesarfo/payments-engine/pkg/idempotency"
	"github.com/thesarfo/payments-engine/pkg/logging"
)

func main() {
	logger := logging.New()

	if err := godotenv.Load(); err != nil {
		logger.Warn().Err(err).Msg("godotenv unavailable, using process environment only")
	}

	cfg, err := config.LoadServerConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("load server config")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("pgx pool initialization failed")
	}
	defer pool.Close()

	accountRepo := account.NewAccountRepository(pool)
	ledgerRepo := ledger.NewLedgerRepository(pool)
	transactionRepo := transaction.NewPostgresRepository(pool)

	svc := account.NewAccountService(accountRepo)
	ledgerSvc := ledger.NewLedger(ledgerRepo)
	accountHandler := handler.NewAccountHandler(svc, ledgerSvc)

	transferSvc := transaction.NewTransferService(transactionRepo, ledgerSvc)
	redisAddr := cfg.RedisAddr
	if redisAddr != "" {
		redisClient := redis.NewClient(&redis.Options{
			Addr: redisAddr,
		})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Warn().Str("redis_addr", redisAddr).Err(err).Msg("redis unavailable; continuing without redis idempotency")
			_ = redisClient.Close()
		} else {
			transferSvc = transaction.NewTransferService(
				transactionRepo,
				ledgerSvc,
				idempotency.NewRedisStore(redisClient),
			)
			defer func() {
				if err := redisClient.Close(); err != nil {
					logger.Warn().Err(err).Msg("redis close failed")
				}
			}()
			logger.Info().Str("redis_addr", redisAddr).Msg("redis idempotency enabled")
		}
	}

	transferHandler := handler.NewTransferHandler(transferSvc)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(apimiddleware.RequestLogger(logger))
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/accounts", accountHandler.CreateAccount)
		r.Get("/accounts/{id}", accountHandler.GetAccountByID)
		r.Get("/accounts/{id}/entries", accountHandler.GetAccountEntries)
		r.Post("/transfers", transferHandler.CreateTransfer)
		r.Get("/transfers/{id}", transferHandler.GetTransferByID)
	})

	addr := cfg.ListenAddr
	logger.Info().Str("listen_addr", addr).Msg("server listening")
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Fatal().Err(err).Msg("http server stopped")
	}

}
