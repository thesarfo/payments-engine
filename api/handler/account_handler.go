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
	"github.com/thesarfo/payments-engine/internal/ledger"
	"github.com/thesarfo/payments-engine/pkg/logctx"
	"github.com/thesarfo/payments-engine/pkg/money"
)

type AccountHandler struct {
	svc       *account.AccountService
	ledgerSvc *ledger.Ledger
}

func NewAccountHandler(svc *account.AccountService, ledgerSvc *ledger.Ledger) *AccountHandler {
	return &AccountHandler{svc: svc, ledgerSvc: ledgerSvc}
}

func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		setRequestError(r, "method_not_allowed", "only POST is supported for this endpoint")
		writeJSON(w, http.StatusMethodNotAllowed, dto.ErrorResponse{Error: "method not allowed"})
		return
	}

	var req dto.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		setRequestError(r, "invalid_json", "request body is not valid JSON")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid JSON body"})
		return
	}

	acc, err := h.svc.CreateAccount(r.Context(), account.CreateAccountInput{
		Name:     req.Name,
		Type:     account.AccountType(strings.ToUpper(strings.TrimSpace(req.Type))),
		Currency: money.Currency(strings.ToUpper(strings.TrimSpace(req.Currency))),
	})
	if err != nil {
		setRequestError(r, "invalid_request", err.Error())
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
		setRequestError(r, "invalid_account_id", "account id is not a valid UUID")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid account id"})
		return
	}

	acc, err := h.svc.GetAccountByID(r.Context(), id)
	if errors.Is(err, account.ErrAccountNotFound) {
		setRequestError(r, "account_not_found", "account was not found for provided id")
		writeJSON(w, http.StatusNotFound, dto.ErrorResponse{Error: "account not found"})
		return
	}
	if err != nil {
		setRequestError(r, "internal_error", "failed to load account by id")
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

func (h *AccountHandler) GetAccountEntries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		setRequestError(r, "invalid_account_id", "account id is not a valid UUID")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid account id"})
		return
	}

	rows, err := h.ledgerSvc.GetAccountEntries(r.Context(), id)
	if errors.Is(err, ledger.ErrAccountNotFound) {
		setRequestError(r, "account_not_found", "account was not found for provided id")
		writeJSON(w, http.StatusNotFound, dto.ErrorResponse{Error: "account not found"})
		return
	}
	if err != nil {
		setRequestError(r, "internal_error", "failed to load ledger entries for account")
		writeJSON(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}

	out := make([]dto.AccountEntryResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, dto.AccountEntryResponse{
			EntryID:          row.EntryID.String(),
			PostedAt:         row.PostedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			EntryDescription: row.EntryDescription,
			Reference:        row.Reference,
			EntryStatus:      row.EntryStatus,
			LineID:           row.LineID.String(),
			LineType:         string(row.LineType),
			Amount:           row.Amount.StringFixed(4),
			LineDescription:  row.LineDescription,
			Sequence:         row.Sequence,
		})
	}

	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func setRequestError(r *http.Request, code, detail string) {
	logctx.SetError(r.Context(), code, detail)
}
