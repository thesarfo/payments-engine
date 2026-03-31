package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/api/dto"
	"github.com/thesarfo/payments-engine/internal/transaction"
)

type fakeAccountByCode struct {
	acc transaction.AccountSnapshot
	err error
}

func (f *fakeAccountByCode) GetAccountByCode(_ context.Context, _ string) (transaction.AccountSnapshot, error) {
	if f.err != nil {
		return transaction.AccountSnapshot{}, f.err
	}
	return f.acc, nil
}

func TestHealthHandler_GetClearingHealth_OK(t *testing.T) {
	t.Setenv("CLEARING_ACCOUNT_CODE", "")
	h := NewHealthHandler(&fakeAccountByCode{
		acc: transaction.AccountSnapshot{
			ID:       uuid.New(),
			Code:     "GL_LIAB_CLEARING",
			Currency: "GHS",
			Balance:  decimal.Zero,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/clearing", nil)
	rec := httptest.NewRecorder()
	h.GetClearingHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var out dto.ClearingHealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "OK" || out.Balance != "0.0000" {
		t.Fatalf("%+v", out)
	}
}

func TestHealthHandler_GetClearingHealth_ALERT_nonZero(t *testing.T) {
	h := NewHealthHandler(&fakeAccountByCode{
		acc: transaction.AccountSnapshot{
			ID:       uuid.New(),
			Currency: "GHS",
			Balance:  decimal.NewFromInt(10),
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/clearing", nil)
	rec := httptest.NewRecorder()
	h.GetClearingHealth(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
	var out dto.ClearingHealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "ALERT" || out.Reason != "clearing_account_balance_non_zero" {
		t.Fatalf("%+v", out)
	}
}

func TestHealthHandler_GetClearingHealth_ALERT_missingAccount(t *testing.T) {
	h := NewHealthHandler(&fakeAccountByCode{err: transaction.ErrAccountNotFound})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/clearing", nil)
	rec := httptest.NewRecorder()
	h.GetClearingHealth(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
	var out dto.ClearingHealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "ALERT" || out.Reason != "clearing_account_not_found" {
		t.Fatalf("%+v", out)
	}
}

func TestHealthHandler_GetClearingHealth_internalError(t *testing.T) {
	h := NewHealthHandler(&fakeAccountByCode{err: errors.New("db down")})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/clearing", nil)
	rec := httptest.NewRecorder()
	h.GetClearingHealth(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
}
