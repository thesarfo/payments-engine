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
	"github.com/thesarfo/payments-engine/internal/audit"
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

type fakeAuditLogger struct {
	rangeFn func(ctx context.Context, entityType string, entityID uuid.UUID, from, to *time.Time) ([]audit.AuditEvent, error)
	getFn   func(ctx context.Context, entityType string, entityID uuid.UUID) ([]audit.AuditEvent, error)
}

func (f *fakeAuditLogger) Log(context.Context, audit.AuditEvent) error { return nil }

func (f *fakeAuditLogger) GetByEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]audit.AuditEvent, error) {
	if f.getFn != nil {
		return f.getFn(ctx, entityType, entityID)
	}
	return nil, nil
}

func (f *fakeAuditLogger) GetByEntityRange(ctx context.Context, entityType string, entityID uuid.UUID, from, to *time.Time) ([]audit.AuditEvent, error) {
	if f.rangeFn != nil {
		return f.rangeFn(ctx, entityType, entityID, from, to)
	}
	return nil, nil
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
	}, nil)

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
	}, nil)

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
	}, nil)

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
	}, nil)

	r := chi.NewRouter()
	r.Get("/api/v1/transfers/{id}", h.GetTransferByID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transfers/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGetTransactionAudit_NotFound(t *testing.T) {
	txID := uuid.New()
	h := NewTransferHandler(&fakeTransferService{
		transferFn: func(context.Context, transaction.TransferRequest) (*transaction.Transaction, error) { return nil, nil },
		getFn: func(context.Context, uuid.UUID) (transaction.Transaction, error) {
			return transaction.Transaction{}, transaction.ErrTransactionNotFound
		},
	}, &fakeAuditLogger{})

	r := chi.NewRouter()
	r.Get("/api/v1/transactions/{id}/audit", h.GetTransactionAudit)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+txID.String()+"/audit", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGetTransactionAudit_InvalidFrom(t *testing.T) {
	txID := uuid.New()
	now := time.Now().UTC()
	h := NewTransferHandler(&fakeTransferService{
		transferFn: func(context.Context, transaction.TransferRequest) (*transaction.Transaction, error) { return nil, nil },
		getFn: func(context.Context, uuid.UUID) (transaction.Transaction, error) {
			return transaction.Transaction{ID: txID, CreatedAt: now}, nil
		},
	}, &fakeAuditLogger{})

	r := chi.NewRouter()
	r.Get("/api/v1/transactions/{id}/audit", h.GetTransactionAudit)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+txID.String()+"/audit?from=not-a-date", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetTransactionAudit_Success(t *testing.T) {
	txID := uuid.New()
	now := time.Now().UTC()
	tFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tTo := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)

	var gotFrom, gotTo *time.Time
	var gotEntity string
	var gotID uuid.UUID

	al := &fakeAuditLogger{
		rangeFn: func(_ context.Context, entityType string, entityID uuid.UUID, from, to *time.Time) ([]audit.AuditEvent, error) {
			gotEntity = entityType
			gotID = entityID
			gotFrom, gotTo = from, to
			return []audit.AuditEvent{
				{ID: 1, EntityType: audit.EntityTransaction, EntityID: txID, EventType: audit.EventTransferInitiated, Actor: "x", Payload: []byte(`{}`), OccurredAt: now},
				{ID: 2, EntityType: audit.EntityTransaction, EntityID: txID, EventType: audit.EventTransferSettled, Actor: "x", Payload: []byte(`{}`), OccurredAt: now.Add(time.Second)},
			}, nil
		},
	}

	h := NewTransferHandler(&fakeTransferService{
		transferFn: func(context.Context, transaction.TransferRequest) (*transaction.Transaction, error) { return nil, nil },
		getFn: func(context.Context, uuid.UUID) (transaction.Transaction, error) {
			return transaction.Transaction{ID: txID, CreatedAt: now}, nil
		},
	}, al)

	r := chi.NewRouter()
	r.Get("/api/v1/transactions/{id}/audit", h.GetTransactionAudit)

	q := "/api/v1/transactions/" + txID.String() + "/audit?from=" + tFrom.UTC().Format(time.RFC3339Nano) + "&to=" + tTo.UTC().Format(time.RFC3339Nano)
	req := httptest.NewRequest(http.MethodGet, q, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotEntity != audit.EntityTransaction || gotID != txID {
		t.Fatalf("unexpected audit query: entity=%s id=%s", gotEntity, gotID)
	}
	if gotFrom == nil || gotTo == nil || !gotFrom.Equal(tFrom) || !gotTo.Equal(tTo) {
		t.Fatalf("from/to mismatch: from=%v to=%v", gotFrom, gotTo)
	}

	var out []dto.AuditEventResponse
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 2 || out[0].EventType != audit.EventTransferInitiated || out[1].EventType != audit.EventTransferSettled {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestGetTransactionAudit_NilAuditLogger(t *testing.T) {
	txID := uuid.New()
	now := time.Now().UTC()
	h := NewTransferHandler(&fakeTransferService{
		transferFn: func(context.Context, transaction.TransferRequest) (*transaction.Transaction, error) { return nil, nil },
		getFn: func(context.Context, uuid.UUID) (transaction.Transaction, error) {
			return transaction.Transaction{ID: txID, CreatedAt: now}, nil
		},
	}, nil)

	r := chi.NewRouter()
	r.Get("/api/v1/transactions/{id}/audit", h.GetTransactionAudit)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+txID.String()+"/audit", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var out []dto.AuditEventResponse
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty audit, got %d", len(out))
	}
}
