package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var (
	ErrAccountNotFound  = errors.New("account not found")
	ErrAccountNotActive = errors.New("account not active")
	ErrCurrencyMismatch = errors.New("currency mismatch")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

type LedgerRepository struct {
	pool *pgxpool.Pool
}

type AccountEntryRow struct {
	EntryID          uuid.UUID
	PostedAt         time.Time
	EntryDescription string
	Reference        string
	EntryStatus      string
	LineID           uuid.UUID
	LineType         LineType
	Amount           decimal.Decimal
	LineDescription  string
	Sequence         int16
}

func NewLedgerRepository(pool *pgxpool.Pool) *LedgerRepository {
	return &LedgerRepository{pool: pool}
}

func (r *LedgerRepository) GetAccountEntryRows(ctx context.Context, accountID uuid.UUID) ([]AccountEntryRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			e.id,
			e.posted_at,
			e.description,
			COALESCE(e.reference, ''),
			e.status,
			l.id,
			l.type,
			l.amount::text,
			COALESCE(l.description, ''),
			l.sequence
		FROM journal_entry_lines l
		JOIN journal_entries e ON e.id = l.entry_id
		WHERE l.account_id = $1
		ORDER BY e.posted_at DESC, l.sequence ASC
	`, accountID)
	if err != nil {
		return nil, fmt.Errorf("query account entry rows: %w", err)
	}
	defer rows.Close()

	out := make([]AccountEntryRow, 0)
	for rows.Next() {
		var (
			row       AccountEntryRow
			lineType  string
			amountStr string
		)
		if err := rows.Scan(
			&row.EntryID,
			&row.PostedAt,
			&row.EntryDescription,
			&row.Reference,
			&row.EntryStatus,
			&row.LineID,
			&lineType,
			&amountStr,
			&row.LineDescription,
			&row.Sequence,
		); err != nil {
			return nil, fmt.Errorf("scan account entry row: %w", err)
		}

		amt, err := decimal.NewFromString(amountStr)
		if err != nil {
			return nil, fmt.Errorf("parse row amount: %w", err)
		}

		row.LineType = LineType(lineType)
		row.Amount = amt
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows account entries: %w", err)
	}

	if len(out) > 0 {
		return out, nil
	}

	var exists int
	err = r.pool.QueryRow(ctx, `SELECT 1 FROM accounts WHERE id = $1`, accountID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("check account existence: %w", err)
	}
	return out, nil
}

// InsertJournalEntry initiates a single transaction that:
// 1. validate header
// 2. lock and verify accounts
// 3. insert header
// 4. insert lines
// 5. compute/apply balance deltas
func (r *LedgerRepository) InsertJournalEntry(ctx context.Context, entry JournalEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := validateEntryHeader(entry); err != nil {
		return err
	}

	accountIDs := collectAccountIDs(entry)

	acctType, acctBalance, err := r.loadAndLockAccounts(ctx, tx, accountIDs, entry.Currency)
	if err != nil {
		return err
	}

	netDelta := computeNetDeltas(entry.Lines, acctType)
	if err := checkSufficientBalances(acctBalance, netDelta); err != nil {
		return err
	}

	entryID, err := insertEntryHeader(ctx, tx, entry)
	if err != nil {
		return err
	}

	if err := insertEntryLines(ctx, tx, entryID, entry.Lines); err != nil {
		return err
	}

	if err := applyBalanceUpdates(ctx, tx, netDelta); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func validateEntryHeader(entry JournalEntry) error {
	if entry.Description == "" {
		return errors.New("description is required")
	}
	if entry.Currency == "" {
		return errors.New("currency is required")
	}
	if entry.PostedBy == "" {
		return errors.New("posted_by is required")
	}
	if len(entry.Lines) == 0 {
		return errors.New("journal entry must have at least one line")
	}
	return nil
}

func collectAccountIDs(entry JournalEntry) []uuid.UUID {
	set := make(map[uuid.UUID]struct{})
	for _, line := range entry.Lines {
		set[line.AccountId] = struct{}{}
	}
	ids := make([]uuid.UUID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids
}

// loadAndLockAccounts selects accounts in consistent UUID order (ORDER BY id)
// and locks them with FOR UPDATE. Ordering prevents deadlocks when concurrent
// transactions touch overlapping account sets. Returns account type and balance
// maps so the caller can check sufficient funds before applying deltas.
func (r *LedgerRepository) loadAndLockAccounts(
	ctx context.Context,
	tx pgx.Tx,
	ids []uuid.UUID,
	expectedCurrency string,
) (acctType map[uuid.UUID]string, acctBalance map[uuid.UUID]decimal.Decimal, _ error) {
	if len(ids) == 0 {
		return nil, nil, errors.New("no accounts to load")
	}

	rows, err := tx.Query(ctx, `
		SELECT id, type, currency, status, balance::text
		FROM accounts
		WHERE id = ANY($1)
		ORDER BY id
		FOR UPDATE
	`, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("select accounts: %w", err)
	}
	defer rows.Close()

	acctType = make(map[uuid.UUID]string, len(ids))
	acctBalance = make(map[uuid.UUID]decimal.Decimal, len(ids))
	acctStatus := make(map[uuid.UUID]string, len(ids))
	acctCurrency := make(map[uuid.UUID]string, len(ids))

	seen := 0
	for rows.Next() {
		var (
			id        uuid.UUID
			typeStr   string
			curStr    string
			statusStr string
			balStr    string
		)
		if err := rows.Scan(&id, &typeStr, &curStr, &statusStr, &balStr); err != nil {
			return nil, nil, fmt.Errorf("scan accounts: %w", err)
		}
		bal, err := decimal.NewFromString(balStr)
		if err != nil {
			return nil, nil, fmt.Errorf("parse balance for account %s: %w", id, err)
		}
		seen++
		acctType[id] = typeStr
		acctBalance[id] = bal
		acctStatus[id] = statusStr
		acctCurrency[id] = curStr
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("accounts rows: %w", err)
	}

	if seen != len(ids) {
		return nil, nil, ErrAccountNotFound
	}
	for _, id := range ids {
		if acctStatus[id] != "ACTIVE" {
			return nil, nil, ErrAccountNotActive
		}
		if acctCurrency[id] != expectedCurrency {
			return nil, nil, ErrCurrencyMismatch
		}
	}

	return acctType, acctBalance, nil
}

// checkSufficientBalances verifies that no account's stored balance will drop
// below zero after applying the computed net deltas. This check runs inside the
// FOR UPDATE lock, so the balance values are authoritative and race-free.
func checkSufficientBalances(
	acctBalance map[uuid.UUID]decimal.Decimal,
	netDelta map[uuid.UUID]decimal.Decimal,
) error {
	for id, delta := range netDelta {
		newBalance := acctBalance[id].Add(delta)
		if newBalance.LessThan(decimal.Zero) {
			return fmt.Errorf("%w: account %s (current %s, delta %s)",
				ErrInsufficientFunds, id, acctBalance[id], delta)
		}
	}
	return nil
}

func insertEntryHeader(ctx context.Context, tx pgx.Tx, entry JournalEntry) (uuid.UUID, error) {
	status := entry.Status
	if status == "" {
		status = EntryStatusPosted
	}
	postedAt := entry.PostedAt
	if postedAt.IsZero() {
		postedAt = time.Now()
	}

	var entryID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO journal_entries (transaction_id, reference, description, currency, status, posted_at, posted_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`,
		nullIfNilUUID(entry.TransactionId),
		entry.Reference,
		entry.Description,
		entry.Currency,
		string(status),
		postedAt,
		entry.PostedBy,
	).Scan(&entryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, errors.New("failed to insert journal entry")
		}
		return uuid.Nil, fmt.Errorf("insert journal entry: %w", err)
	}
	return entryID, nil
}

func insertEntryLines(ctx context.Context, tx pgx.Tx, entryID uuid.UUID, lines []JournalEntryLine) error {
	for _, line := range lines {
		if !line.Amount.GreaterThan(decimal.Zero) {
			return errors.New("line amount must be > 0")
		}
		if line.Type != LineTypeDebit && line.Type != LineTypeCredit {
			return errors.New("invalid line type")
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO journal_entry_lines (entry_id, account_id, type, amount, description, sequence)
			VALUES ($1, $2, $3, $4, $5, $6)
		`,
			entryID,
			line.AccountId,
			string(line.Type),
			line.Amount,
			line.Description,
			line.Sequence,
		); err != nil {
			return fmt.Errorf("insert journal_entry_lines: %w", err)
		}
	}
	return nil
}

func computeNetDeltas(
	lines []JournalEntryLine,
	acctType map[uuid.UUID]string,
) map[uuid.UUID]decimal.Decimal {
	netDelta := make(map[uuid.UUID]decimal.Decimal, len(acctType))

	for _, line := range lines {
		aType := acctType[line.AccountId]
		var delta decimal.Decimal

		switch line.Type {
		case LineTypeDebit:
			switch aType {
			case "ASSET", "EXPENSE":
				delta = line.Amount
			default:
				delta = line.Amount.Neg()
			}
		case LineTypeCredit:
			switch aType {
			case "LIABILITY", "EQUITY", "INCOME":
				delta = line.Amount
			default:
				delta = line.Amount.Neg()
			}
		}

		netDelta[line.AccountId] = netDelta[line.AccountId].Add(delta)
	}

	return netDelta
}

func applyBalanceUpdates(ctx context.Context, tx pgx.Tx, deltas map[uuid.UUID]decimal.Decimal) error {
	for accountID, delta := range deltas {
		if _, err := tx.Exec(ctx, `
			UPDATE accounts
			SET balance = balance + $1,
			    version = version + 1
			WHERE id = $2
		`, delta, accountID); err != nil {
			return fmt.Errorf("update balance: %w", err)
		}
	}
	return nil
}

// nullIfNilUUID returns nil for uuid.Nil so INSERT uses NULL, otherwise the UUID.
func nullIfNilUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
