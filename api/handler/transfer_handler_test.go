package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/api/dto"
	"github.com/thesarfo/payments-engine/internal/transaction"
)

type fakeTransferService struct {
	transferFn func(ctx context.Context, req transaction.TransferRequest) (*transaction.Transaction, error)
	getFn      func(ctx context.Context, txID uuid.UUID) (transaction.Transaction, error)
}

func (f *fakeTransferService) Transfer(ctx context.Context, req transaction.TransferRequest) (*transaction.Transaction, error) {
	return f.transferFn(ctx, req)
}

func (f *fakeTransferService) GetTransactionByID(ctx context.Context, txID uuid.UUID) (transaction.Transaction, error) {
	return f.getFn(ctx, txID)
}

func TestCreateTransfer_RequiresIdempotencyHeader(t *testing.T) {
	h := NewTransferHandler(&fakeTransferService{
		transferFn: func(context.Context, transaction.TransferRequest) (*transaction.Transaction, error) {
			t.Fatal("service should not be called when header is missing")
			return nil, nil
		},
		getFn: func(context.Context, uuid.UUID) (transaction.Transaction, error) {
			return transaction.Transaction{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transfers", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.CreateTransfer(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateTransfer_ReturnsTransactionJSON(t *testing.T) {
	txID := uuid.New()
	fromID := uuid.New()
	toID := uuid.New()
	journalID := uuid.New()
	now := time.Now().UTC()

	h := NewTransferHandler(&fakeTransferService{
		transferFn: func(_ context.Context, req transaction.TransferRequest) (*transaction.Transaction, error) {
			if req.IdempotencyKey != "idem-123" {
				t.Fatalf("unexpected idempotency key: %s", req.IdempotencyKey)
			}
			return &transaction.Transaction{
				ID:             txID,
				IdempotencyKey: req.IdempotencyKey,
				FromAccountID:  fromID,
				ToAccountID:    toID,
				Amount:         decimal.RequireFromString("100.0000"),
				Currency:       "GHS",
				Status:         transaction.TxStatusSettled,
				JournalEntryId: &journalID,
				CreatedAt:      now,
				UpdatedAt:      now,
				SettledAt:      &now,
			}, nil
		},
		getFn: func(context.Context, uuid.UUID) (transaction.Transaction, error) {
			return transaction.Transaction{}, nil
		},
	})

	body := `{"from_account_id":"` + fromID.String() + `","to_account_id":"` + toID.String() + `","amount":"100.0000","currency":"ghs"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transfers", strings.NewReader(body))
	req.Header.Set("X-Idempotency-Key", "idem-123")
	rr := httptest.NewRecorder()

	h.CreateTransfer(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var resp dto.TransactionResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != txID.String() {
		t.Fatalf("expected tx id %s, got %s", txID, resp.ID)
	}
	if resp.Status != string(transaction.TxStatusSettled) {
		t.Fatalf("expected status SETTLED, got %s", resp.Status)
	}
	if resp.Amount != "100.0000" {
		t.Fatalf("expected amount string 100.0000, got %s", resp.Amount)
	}
	if resp.JournalEntryID == nil || *resp.JournalEntryID != journalID.String() {
		t.Fatalf("expected journal_entry_id %s, got %+v", journalID, resp.JournalEntryID)
	}
}

func TestGetTransferByID(t *testing.T) {
	txID := uuid.New()
	now := time.Now().UTC()
	h := NewTransferHandler(&fakeTransferService{
		transferFn: func(context.Context, transaction.TransferRequest) (*transaction.Transaction, error) {
			return nil, nil
		},
		getFn: func(_ context.Context, id uuid.UUID) (transaction.Transaction, error) {
			if id != txID {
				t.Fatalf("expected id %s, got %s", txID, id)
			}
			return transaction.Transaction{
				ID:             txID,
				IdempotencyKey: "idem-poll",
				Amount:         decimal.RequireFromString("20.0000"),
				Currency:       "GHS",
				Status:         transaction.TxStatusProcessing,
				CreatedAt:      now,
				UpdatedAt:      now,
			}, nil
		},
	})

	r := chi.NewRouter()
	r.Get("/api/v1/transfers/{id}", h.GetTransferByID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transfers/"+txID.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestGetTransferByID_NotFound(t *testing.T) {
	h := NewTransferHandler(&fakeTransferService{
		transferFn: func(context.Context, transaction.TransferRequest) (*transaction.Transaction, error) {
			return nil, nil
		},
		getFn: func(_ context.Context, _ uuid.UUID) (transaction.Transaction, error) {
			return transaction.Transaction{}, transaction.ErrTransactionNotFound
		},
	})

	r := chi.NewRouter()
	r.Get("/api/v1/transfers/{id}", h.GetTransferByID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transfers/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
