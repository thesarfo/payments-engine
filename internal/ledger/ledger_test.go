package ledger

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var invariantDB *pgxpool.Pool

func TestMain(m *testing.M) {
	url := os.Getenv("DATABASE_URL")
	if url != "" {
		pool, err := pgxpool.New(context.Background(), url)
		if err == nil {
			invariantDB = pool
			defer invariantDB.Close()
		}
	}
	os.Exit(m.Run())
}

func assertInvariantAfterTest(t *testing.T) {
	t.Helper()
	if invariantDB == nil {
		return
	}
	t.Cleanup(func() {
		AssertLedgerBalanced(t, invariantDB)
	})
}

type fakeRepo struct {
	err   error
	calls int
	last  JournalEntry
}

func (f *fakeRepo) InsertJournalEntry(_ context.Context, entry JournalEntry) error {
	f.calls++
	f.last = entry
	return f.err
}

func TestLedger_PostJournalEntry_Balanced(t *testing.T) {
	assertInvariantAfterTest(t)
	repo := &fakeRepo{}
	l := NewLedger(repo)

	entry := JournalEntry{
		ID:          uuid.New(),
		Description: "balanced entry",
		Currency:    "GHS",
		PostedBy:    "test",
		Lines: []JournalEntryLine{
			{
				AccountId: uuid.New(),
				Type:      LineTypeDebit,
				Amount:    decimal.NewFromInt(100),
			},
			{
				AccountId: uuid.New(),
				Type:      LineTypeCredit,
				Amount:    decimal.NewFromInt(100),
			},
		},
	}

	if err := l.PostJournalEntry(context.Background(), entry); err != nil {
		t.Fatalf("PostJournalEntry() unexpected error: %v", err)
	}
	if repo.calls != 1 {
		t.Fatalf("expected repo.InsertJournalEntry to be called once, got %d", repo.calls)
	}
}

func TestLedger_PostJournalEntry_Unbalanced(t *testing.T) {
	assertInvariantAfterTest(t)
	repo := &fakeRepo{}
	l := NewLedger(repo)

	entry := JournalEntry{
		ID:          uuid.New(),
		Description: "unbalanced entry",
		Currency:    "GHS",
		PostedBy:    "test",
		Lines: []JournalEntryLine{
			{
				AccountId: uuid.New(),
				Type:      LineTypeDebit,
				Amount:    decimal.NewFromInt(100),
			},
			{
				AccountId: uuid.New(),
				Type:      LineTypeCredit,
				Amount:    decimal.NewFromInt(90),
			},
		},
	}

	err := l.PostJournalEntry(context.Background(), entry)
	if !errors.Is(err, ErrUnbalancedEntry) {
		t.Fatalf("expected ErrUnbalancedEntry, got %v", err)
	}
	if repo.calls != 0 {
		t.Fatalf("expected repo.InsertJournalEntry not to be called, got %d calls", repo.calls)
	}
}

func TestLedger_PostJournalEntry_InvalidLineType(t *testing.T) {
	assertInvariantAfterTest(t)
	repo := &fakeRepo{}
	l := NewLedger(repo)

	entry := JournalEntry{
		ID:          uuid.New(),
		Description: "invalid line type",
		Currency:    "GHS",
		PostedBy:    "test",
		Lines: []JournalEntryLine{
			{
				AccountId: uuid.New(),
				Type:      "UNKNOWN",
				Amount:    decimal.NewFromInt(100),
			},
		},
	}

	err := l.PostJournalEntry(context.Background(), entry)
	if err == nil {
		t.Fatal("expected error for invalid line type, got nil")
	}
	if errors.Is(err, ErrUnbalancedEntry) {
		t.Fatalf("expected generic invalid line type error, got ErrUnbalancedEntry")
	}
	if repo.calls != 0 {
		t.Fatalf("expected repo.InsertJournalEntry not to be called, got %d calls", repo.calls)
	}
}
