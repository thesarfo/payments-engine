package ledger

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

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
	repo := &fakeRepo{}
	l := NewLedger(repo)

	entry := JournalEntry{
		ID:          uuid.New(),
		Description: "balanced entry",
		Currency:    "USD",
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
	repo := &fakeRepo{}
	l := NewLedger(repo)

	entry := JournalEntry{
		ID:          uuid.New(),
		Description: "unbalanced entry",
		Currency:    "USD",
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
	repo := &fakeRepo{}
	l := NewLedger(repo)

	entry := JournalEntry{
		ID:          uuid.New(),
		Description: "invalid line type",
		Currency:    "USD",
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

