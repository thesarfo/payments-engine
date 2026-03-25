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

	"github.com/thesarfo/payments-engine/api/handler"
	apimiddleware "github.com/thesarfo/payments-engine/api/middleware"
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

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(apimiddleware.RequestLogger)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/accounts", h.CreateAccount)
		r.Get("/accounts/{id}", h.GetAccountByID)
	})

	addr := ":8080"
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}

}	