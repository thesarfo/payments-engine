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
	"github.com/thesarfo/payments-engine/internal/audit"
	"github.com/thesarfo/payments-engine/internal/ledger"
	"github.com/thesarfo/payments-engine/pkg/logctx"
	"github.com/thesarfo/payments-engine/pkg/money"
)

type AccountHandler struct {
	svc         *account.AccountService
	ledgerSvc   *ledger.Ledger
	auditLogger audit.Logger
}

func NewAccountHandler(svc *account.AccountService, ledgerSvc *ledger.Ledger, auditLogger audit.Logger) *AccountHandler {
	return &AccountHandler{svc: svc, ledgerSvc: ledgerSvc, auditLogger: auditLogger}
}

// CreateAccount creates a new account in the chart of accounts.
//
//	@Summary		Create an account
//	@Tags			accounts
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.CreateAccountRequest	true	"Account details"
//	@Success		201		{object}	dto.AccountResponse
//	@Failure		400		{object}	dto.ErrorResponse
//	@Failure		500		{object}	dto.ErrorResponse
//	@Router			/accounts [post]
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

// GetAccountByID fetches an account by its UUID.
//
//	@Summary		Get account by ID
//	@Tags			accounts
//	@Produce		json
//	@Param			id	path		string	true	"Account UUID"
//	@Success		200	{object}	dto.AccountResponse
//	@Failure		400	{object}	dto.ErrorResponse
//	@Failure		404	{object}	dto.ErrorResponse
//	@Failure		500	{object}	dto.ErrorResponse
//	@Router			/accounts/{id} [get]
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

// UpdateAccountStatus freezes, unfreezes, or closes an account.
//
//	@Summary		Update account status
//	@Tags			accounts
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string							true	"Account UUID"
//	@Param			request	body		dto.UpdateAccountStatusRequest	true	"New status"
//	@Success		200		{object}	dto.AccountResponse
//	@Failure		400		{object}	dto.ErrorResponse
//	@Failure		404		{object}	dto.ErrorResponse
//	@Failure		500		{object}	dto.ErrorResponse
//	@Router			/accounts/{id} [patch]
func (h *AccountHandler) UpdateAccountStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		setRequestError(r, "invalid_account_id", "account id is not a valid UUID")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid account id"})
		return
	}

	var req dto.UpdateAccountStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		setRequestError(r, "invalid_json", "request body is not valid JSON")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid JSON body"})
		return
	}

	status := strings.ToUpper(strings.TrimSpace(req.Status))
	switch status {
	case "ACTIVE", "FROZEN", "CLOSED":
	default:
		setRequestError(r, "invalid_status", "status must be one of ACTIVE, FROZEN, CLOSED")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "status must be one of ACTIVE, FROZEN, CLOSED"})
		return
	}

	acc, err := h.svc.UpdateAccountStatus(r.Context(), id, account.AccountStatus(status))
	if errors.Is(err, account.ErrAccountNotFound) {
		setRequestError(r, "account_not_found", "account was not found for provided id")
		writeJSON(w, http.StatusNotFound, dto.ErrorResponse{Error: "account not found"})
		return
	}
	if errors.Is(err, account.ErrInvalidStatusTransition) {
		setRequestError(r, "invalid_status_transition", err.Error())
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err != nil {
		setRequestError(r, "internal_error", "failed to update account status")
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

// GetAccountEntries returns all journal entry lines posted to an account.
//
//	@Summary		List journal entries
//	@Tags			accounts
//	@Produce		json
//	@Param			id	path		string	true	"Account UUID"
//	@Success		200	{array}		dto.AccountEntryResponse
//	@Failure		400	{object}	dto.ErrorResponse
//	@Failure		404	{object}	dto.ErrorResponse
//	@Failure		500	{object}	dto.ErrorResponse
//	@Router			/accounts/{id}/entries [get]
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

// GetAccountAuditLog returns the immutable audit trail for an account.
//
//	@Summary		Account audit log
//	@Tags			accounts
//	@Produce		json
//	@Param			id		path		string	true	"Account UUID"
//	@Param			from	query		string	false	"RFC3339 start time (inclusive)"	example(2024-01-01T00:00:00Z)
//	@Param			to		query		string	false	"RFC3339 end time (inclusive)"		example(2024-12-31T23:59:59Z)
//	@Success		200		{array}		dto.AuditEventResponse
//	@Failure		400		{object}	dto.ErrorResponse
//	@Failure		500		{object}	dto.ErrorResponse
//	@Router			/accounts/{id}/audit [get]
func (h *AccountHandler) GetAccountAuditLog(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		setRequestError(r, "invalid_account_id", "account id is not a valid UUID")
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid account id"})
		return
	}

	if h.auditLogger == nil {
		writeJSON(w, http.StatusOK, []dto.AuditEventResponse{})
		return
	}

	from, to, err := parseAuditTimeQuery(r)
	if err != nil {
		setRequestError(r, "invalid_query", err.Error())
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	events, err := h.auditLogger.GetByEntityRange(r.Context(), audit.EntityAccount, id, from, to)
	if err != nil {
		setRequestError(r, "internal_error", "failed to load audit log for account")
		writeJSON(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, auditEventsToResponse(events))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func setRequestError(r *http.Request, code, detail string) {
	logctx.SetError(r.Context(), code, detail)
}
