package settlement

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/internal/transaction"
)

func TestNetPositions_AB_nets_to_directed_flow(t *testing.T) {
	a := uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	b := uuid.MustParse("00000000-0000-0000-0000-0000000000b2")
	hundred := decimal.NewFromInt(100)
	sixty := decimal.NewFromInt(60)

	txs := []transaction.Transaction{
		{
			ID:            uuid.New(),
			FromAccountID: a,
			ToAccountID:   b,
			Amount:        hundred,
			Currency:      "GHS",
			Status:        transaction.TxStatusSettled,
		},
		{
			ID:            uuid.New(),
			FromAccountID: b,
			ToAccountID:   a,
			Amount:        sixty,
			Currency:      "GHS",
			Status:        transaction.TxStatusSettled,
		},
	}

	pos := NetPositions(txs)
	if len(pos) != 1 {
		t.Fatalf("expected 1 net position, got %d", len(pos))
	}
	p := pos[0]
	if p.FromAccountID != a || p.ToAccountID != b {
		t.Fatalf("unexpected direction: from %s to %s", p.FromAccountID, p.ToAccountID)
	}
	if !p.NetAmount.Equal(decimal.NewFromInt(40)) {
		t.Fatalf("net amount %s", p.NetAmount)
	}
	if p.TransactionCount != 2 {
		t.Fatalf("count %d", p.TransactionCount)
	}
	if p.Currency != "GHS" {
		t.Fatalf("currency %s", p.Currency)
	}
}

func TestNetPositions_opposite_equal_nets_zero_omitted(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	amt := decimal.NewFromInt(50)
	txs := []transaction.Transaction{
		{FromAccountID: a, ToAccountID: b, Amount: amt, Currency: "GHS"},
		{FromAccountID: b, ToAccountID: a, Amount: amt, Currency: "GHS"},
	}
	pos := NetPositions(txs)
	if len(pos) != 0 {
		t.Fatalf("expected no positions, got %d", len(pos))
	}
}

func TestNetPositions_separate_currencies(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	txs := []transaction.Transaction{
		{FromAccountID: a, ToAccountID: b, Amount: decimal.NewFromInt(10), Currency: "GHS"},
		{FromAccountID: a, ToAccountID: b, Amount: decimal.NewFromInt(5), Currency: "USD"},
	}
	pos := NetPositions(txs)
	if len(pos) != 2 {
		t.Fatalf("got %d", len(pos))
	}
}

func TestSettlementDayBounds_used_by_repository(t *testing.T) {
	loc := time.FixedZone("TST", 0)
	day := time.Date(2026, 3, 15, 15, 30, 0, 0, loc)
	start, end := transaction.SettlementDayBounds(day, loc)
	if !start.Equal(time.Date(2026, 3, 15, 0, 0, 0, 0, loc)) {
		t.Fatalf("start %v", start)
	}
	if !end.Equal(start.Add(24 * time.Hour)) {
		t.Fatalf("end %v", end)
	}
}
