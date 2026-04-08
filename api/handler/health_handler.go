package handler

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/thesarfo/payments-engine/api/dto"
	"github.com/thesarfo/payments-engine/internal/transaction"
	"github.com/thesarfo/payments-engine/pkg/logctx"
)

type AccountByCodeReader interface {
	GetAccountByCode(ctx context.Context, code string) (transaction.AccountSnapshot, error)
}

type HealthHandler struct {
	accounts AccountByCodeReader
	code     string
}

func NewHealthHandler(accounts AccountByCodeReader) *HealthHandler {
	code := strings.TrimSpace(os.Getenv("CLEARING_ACCOUNT_CODE"))
	if code == "" {
		code = transaction.DefaultClearingCode
	}
	return &HealthHandler{accounts: accounts, code: code}
}

// GetClearingHealth checks whether the internal clearing account has a zero balance.
//
//	@Summary		Clearing account health
//	@Description	A non-zero balance indicates a double-entry invariant violation — a transfer settled without fully clearing. Returns 200 when healthy, 503 otherwise.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	dto.ClearingHealthResponse
//	@Failure		503	{object}	dto.ClearingHealthResponse
//	@Router			/health/clearing [get]
func (h *HealthHandler) GetClearingHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		setRequestError(r, "method_not_allowed", "only GET is supported for this endpoint")
		writeJSON(w, http.StatusMethodNotAllowed, dto.ErrorResponse{Error: "method not allowed"})
		return
	}

	acc, err := h.accounts.GetAccountByCode(r.Context(), h.code)
	if err != nil {
		if errors.Is(err, transaction.ErrAccountNotFound) {
			setRequestError(r, "clearing_account_not_found", "clearing account not found for configured code")
			writeJSON(w, http.StatusServiceUnavailable, dto.ClearingHealthResponse{
				Status:              "ALERT",
				ClearingAccountCode: h.code,
				Balance:             "",
				Reason:              "clearing_account_not_found",
			})
			return
		}
		setRequestError(r, "internal_error", "failed to load clearing account")
		logctx.SetError(r.Context(), "internal_error", err.Error())
		writeJSON(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}

	balStr := acc.Balance.StringFixed(4)
	resp := dto.ClearingHealthResponse{
		ClearingAccountCode: h.code,
		Currency:            acc.Currency,
		Balance:             balStr,
	}

	if acc.Balance.IsZero() {
		resp.Status = "OK"
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp.Status = "ALERT"
	resp.Reason = "clearing_account_balance_non_zero"
	setRequestError(r, "clearing_invariant_violation", "clearing account balance is non-zero")
	writeJSON(w, http.StatusServiceUnavailable, resp)
}
