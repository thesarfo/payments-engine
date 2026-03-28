package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/api/dto"
	"github.com/thesarfo/payments-engine/internal/transaction"
)

const idempotencyHeader = "X-Idempotency-Key"

type transferService interface {
	Transfer(ctx context.Context, req transaction.TransferRequest) (*transaction.Transaction, error)
	GetTransactionByID(ctx context.Context, txID uuid.UUID) (transaction.Transaction, error)
}

type TransferHandler struct {
	svc transferService
}

func NewTransferHandler(svc transferService) *TransferHandler {
	return &TransferHandler{svc: svc}
}

func (h *TransferHandler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := strings.TrimSpace(r.Header.Get(idempotencyHeader))
	if idempotencyKey == "" {
		setRequestError(r, "missing_idempotency_header", "X-Idempotency-Key header is required")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "X-Idempotency-Key header is required"})
		return
	}

	var req dto.CreateTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		setRequestError(r, "invalid_json", "request body is not valid JSON")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid JSON body"})
		return
	}

	fromID, err := uuid.Parse(strings.TrimSpace(req.FromAccountID))
	if err != nil {
		setRequestError(r, "invalid_from_account_id", "from_account_id is not a valid UUID")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid from_account_id"})
		return
	}
	toID, err := uuid.Parse(strings.TrimSpace(req.ToAccountID))
	if err != nil {
		setRequestError(r, "invalid_to_account_id", "to_account_id is not a valid UUID")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid to_account_id"})
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		setRequestError(r, "invalid_amount", "amount must be a valid decimal number")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid amount"})
		return
	}

	normalizedCurrency := strings.ToUpper(strings.TrimSpace(req.Currency))
	tx, err := h.svc.Transfer(r.Context(), transaction.TransferRequest{
		IdempotencyKey: idempotencyKey,
		FromAccountId:  fromID,
		ToAccountId:    toID,
		Amount:         amount,
		Currency:       normalizedCurrency,
		Rail:           req.Rail,
		Description:    req.Description,
	})
	if err != nil {
		h.writeTransferError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, dto.NewTransactionResponse(*tx))
}

func (h *TransferHandler) GetTransferByID(w http.ResponseWriter, r *http.Request) {
	txID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		setRequestError(r, "invalid_transfer_id", "transfer id is not a valid UUID")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid transfer id"})
		return
	}

	tx, err := h.svc.GetTransactionByID(r.Context(), txID)
	if errors.Is(err, transaction.ErrTransactionNotFound) {
		setRequestError(r, "transfer_not_found", "transfer was not found for provided id")
		writeJSON(w, http.StatusNotFound, dto.ErrorResponse{Error: "transfer not found"})
		return
	}
	if err != nil {
		setRequestError(r, "internal_error", "failed to load transfer by id")
		writeJSON(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, dto.NewTransactionResponse(tx))
}

func (h *TransferHandler) writeTransferError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, transaction.ErrInvalidTransfer):
		setRequestError(r, "invalid_transfer", err.Error())
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
	case errors.Is(err, transaction.ErrInsufficientFunds):
		setRequestError(r, "insufficient_funds", err.Error())
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
	case errors.Is(err, transaction.ErrCurrencyMismatch):
		setRequestError(r, "currency_mismatch", err.Error())
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
	case errors.Is(err, transaction.ErrClearingAccountNotFound):
		setRequestError(r, "clearing_account_not_found", err.Error())
		writeJSON(w, http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
	case errors.Is(err, transaction.ErrAccountNotFound):
		setRequestError(r, "account_not_found", err.Error())
		writeJSON(w, http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
	case errors.Is(err, transaction.ErrTransferInProgress):
		setRequestError(r, "transfer_in_progress", err.Error())
		writeJSON(w, http.StatusConflict, dto.ErrorResponse{Error: err.Error()})
	default:
		setRequestError(r, "internal_error", err.Error())
		writeJSON(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
	}
}
