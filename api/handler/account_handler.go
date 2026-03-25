package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/thesarfo/payments-engine/api/dto"
	"github.com/thesarfo/payments-engine/internal/account"
	"github.com/thesarfo/payments-engine/pkg/money"
)

type AccountHandler struct {
	svc *account.Service
}

func NewAccountHandler(svc *account.Service) *AccountHandler {
	return &AccountHandler{svc: svc}
}

func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, dto.ErrorResponse{Error: "method not allowed"})
		return
	}

	var req dto.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid JSON body"})
		return
	}

	acc, err := h.svc.CreateAccount(r.Context(), account.CreateAccountInput{
		Name:     req.Name,
		Type:     account.AccountType(strings.ToUpper(strings.TrimSpace(req.Type))),
		Currency: money.Currency(strings.ToUpper(strings.TrimSpace(req.Currency))),
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, dto.AccountResponse{
		ID:       acc.ID.String(),
		Name:     acc.Name,
		Type:     string(acc.Type),
		Currency: string(acc.Currency),
		Balance:  acc.Balance.StringFixed(4),
		Status:   string(acc.Status),
		Version:  acc.Version,
	})
}

func (h *AccountHandler) GetAccountByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid account id"})
		return
	}

	acc, err := h.svc.GetAccountByID(r.Context(), id)
	if errors.Is(err, account.ErrAccountNotFound) {
		writeJSON(w, http.StatusNotFound, dto.ErrorResponse{Error: "account not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, dto.AccountResponse{
		ID:       acc.ID.String(),
		Name:     acc.Name,
		Type:     string(acc.Type),
		Currency: string(acc.Currency),
		Balance:  acc.Balance.StringFixed(4),
		Status:   string(acc.Status),
		Version:  acc.Version,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
