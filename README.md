# Ledgr

Ledgr provides a ledger and transfer infrastructure for products that need reliable internal money movement.

It provides account management, double-entry posting, and idempotent transfer orchestration behind a clean interface.

## Features

- **Double-Entry Ledger**: every journal post is validated as balanced (total debits = total credits) before persistence.
- **Immutable Audit Trail**: journal entries and lines are append-only in service behavior and remain queryable for full transfer traceability.
- **Chart of Accounts**: hierarchical accounts with support for asset, liability, equity, income, and expense account classes.
- **Account Lifecycle**: freeze, unfreeze, or permanently close accounts. 
- **Transfer Orchestration**: lifecycle management with deterministic state transitions.
- **Safe Retries**: idempotent transfer submission, with optional Redis-backed deduplication acceleration.
- **Trial Balance**: returns per-account debit/credit totals and net; response includes `balanced` and `net_total` (should be zero when the ledger is consistent).
- **Settlement Netting**: loads `SETTLED` transfers for a calendar day, nets flows per account pair and atomically reconciles them.
- **Clearing Account Health**: checks the health of the clearing account after a period — point a probe or cron at the endpoint to ensure the clearing balance is zero after settlement.



## Quick start (Docker)

```bash
docker compose up
```

This starts Postgres, Redis, runs migrations automatically, and serves the API on `:8080`.

API Docs: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)



## Local development

**Prerequisites:** Go 1.23+, PostgreSQL, [`migrate`](https://github.com/golang-migrate/migrate) CLI. Redis is optional.

```bash
cp .env.example .env          
make migrate
make seed
go run ./cmd/server/main.go
```
