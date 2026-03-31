package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/api/dto"
	"github.com/thesarfo/payments-engine/internal/ledger"
)

type fakeTrialBalanceRepo struct{}

func (f *fakeTrialBalanceRepo) InsertJournalEntry(_ context.Context, _ ledger.JournalEntry) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (f *fakeTrialBalanceRepo) GetTrialBalance(_ context.Context, currency string) (ledger.TrialBalance, error) {
	a := uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	b := uuid.MustParse("00000000-0000-0000-0000-0000000000b2")
	hundred := decimal.NewFromInt(100)
	return ledger.TrialBalance{
		Currency: currency,
		Rows: []ledger.TrialBalanceLine{
			{AccountID: a, Code: "A", Name: "one", TotalDebits: hundred, TotalCredits: decimal.Zero, Net: hundred},
			{AccountID: b, Code: "B", Name: "two", TotalDebits: decimal.Zero, TotalCredits: hundred, Net: hundred.Neg()},
		},
		Balanced: true,
		NetTotal: decimal.Zero,
	}, nil
}

func TestLedgerHandler_GetTrialBalance_OK(t *testing.T) {
	h := NewLedgerHandler(ledger.NewLedger(&fakeTrialBalanceRepo{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledger/trial-balance?currency=GHS", nil)
	rec := httptest.NewRecorder()
	h.GetTrialBalance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var out dto.TrialBalanceResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Currency != "GHS" {
		t.Fatalf("currency %q", out.Currency)
	}
	if !out.Balanced || out.NetTotal != "0.0000" {
		t.Fatalf("balanced=%v net_total=%q", out.Balanced, out.NetTotal)
	}
	if len(out.Rows) != 2 {
		t.Fatalf("rows %d", len(out.Rows))
	}
}

func TestLedgerHandler_GetTrialBalance_MissingCurrency(t *testing.T) {
	h := NewLedgerHandler(ledger.NewLedger(&fakeTrialBalanceRepo{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledger/trial-balance", nil)
	rec := httptest.NewRecorder()
	h.GetTrialBalance(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestLedgerHandler_GetTrialBalance_InvalidCurrency(t *testing.T) {
	h := NewLedgerHandler(ledger.NewLedger(&fakeTrialBalanceRepo{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledger/trial-balance?currency=GH", nil)
	rec := httptest.NewRecorder()
	h.GetTrialBalance(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestParseRequiredCurrencyQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x?currency=ghs", nil)
	c, err := parseRequiredCurrencyQuery(req)
	if err != nil || c != "GHS" {
		t.Fatalf("got %q err %v", c, err)
	}
}
