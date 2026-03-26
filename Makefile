#
# Without GNU make (e.g. PowerShell), run the recipe line manually.
#
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/payments_engine?sslmode=disable

.PHONY: migrate
migrate:
	migrate -path ./migrations -database "$(DATABASE_URL)" up

.PHONY: seed
seed:
	go run ./cmd/seed/main.go
