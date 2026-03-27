# Ledgr

Ledgr provides a ledger and transfer infrastructure for products that need reliable internal money movement.

It provides account management, double-entry posting, and idempotent transfer orchestration behind a clean API.

## Product capabilities

- **Double-Entry Ledger**: every journal post is validated as balanced (total debits = total credits) before persistence.
- **Immutable Audit Trail**: journal entries and lines are append-only in service behavior and remain queryable for full transfer traceability.
- **Chart of Accounts**: hierarchical accounts with support for asset, liability, equity, income, and expense account classes.
- **Multi-Currency Support (Partial)**: accounts are currency-scoped and transfers enforce currency consistency; cross-currency FX conversion is not yet implemented.
- **Transfer Orchestration**: lifecycle management with deterministic state transitions (`PENDING -> PROCESSING -> SETTLED`).
- **Safe Retries**: idempotent transfer submission via `X-Idempotency-Key`, with optional Redis-backed deduplication acceleration.


<!-- 
## Local deployment

### 1) Start infrastructure

```bash
docker compose up -d
```

Services:

- PostgreSQL: `localhost:5433`
- Redis: `localhost:6379`

### 2) Configure runtime environment

PowerShell:

```powershell
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5433/payments_engine?sslmode=disable"
$env:REDIS_ADDR="localhost:6379"
```

### 3) Run migrations via make or:

```bash
migrate -path ./migrations -database "$DATABASE_URL" up
```

Install migrate if missing:

```bash
go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### 4) Seed system accounts

```bash
go run ./cmd/seed/main.go
```

### 5) Start the service

```bash
go run ./cmd/server/main.go
```


## Integration example

```bash
curl -X POST http://localhost:8080/api/v1/transfers \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: transfer-001" \
  -d '{"from_account_id":"<FROM_UUID>","to_account_id":"<TO_UUID>","amount":"25.0000","currency":"GHS","rail":"INTERNAL","description":"P2P transfer"}'
``` -->

## Transfer guarantees

- Positive, same-currency amounts are enforced before processing
- Source and destination accounts must be `ACTIVE`
- Clearing account `GL_LIAB_CLEARING` is required for posting flow
- Internal rail posts two balanced journal entries:
  - source account -> clearing account
  - clearing account -> destination account
- Duplicate idempotency keys return prior result, or conflict if currently in progress

## Roadmap

- **Hold mechanism (Planned)**: reserve funds separately from ledger balance for pending or authorization-first workflows.
- **Trial balance reports (Planned)**: real-time debit/credit aggregate reporting for operational and finance controls.