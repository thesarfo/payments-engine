package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/thesarfo/payments-engine/api/dto"
	"github.com/thesarfo/payments-engine/internal/ledger"
)

type LedgerHandler struct {
	ledgerSvc *ledger.Ledger
}

func NewLedgerHandler(ledgerSvc *ledger.Ledger) *LedgerHandler {
	return &LedgerHandler{ledgerSvc: ledgerSvc}
}

func (h *LedgerHandler) GetTrialBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		setRequestError(r, "method_not_allowed", "only GET is supported for this endpoint")
		writeJSON(w, http.StatusMethodNotAllowed, dto.ErrorResponse{Error: "method not allowed"})
		return
	}

	currency, err := parseRequiredCurrencyQuery(r)
	if err != nil {
		setRequestError(r, "invalid_query", err.Error())
		writeJSON(w, http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	tb, err := h.ledgerSvc.GetTrialBalance(r.Context(), currency)
	if err != nil {
		if errors.Is(err, ledger.ErrTrialBalanceQueryNotSupported) {
			setRequestError(r, "internal_error", "trial balance is not available")
			writeJSON(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
			return
		}
		setRequestError(r, "internal_error", "failed to load trial balance")
		writeJSON(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}

	rows := make([]dto.TrialBalanceRow, 0, len(tb.Rows))
	for _, line := range tb.Rows {
		rows = append(rows, dto.TrialBalanceRow{
			AccountID:    line.AccountID.String(),
			Code:         line.Code,
			Name:         line.Name,
			TotalDebits:  line.TotalDebits.StringFixed(4),
			TotalCredits: line.TotalCredits.StringFixed(4),
			Net:          line.Net.StringFixed(4),
		})
	}

	writeJSON(w, http.StatusOK, dto.TrialBalanceResponse{
		Currency: tb.Currency,
		Rows:     rows,
		Balanced: tb.Balanced,
		NetTotal: tb.NetTotal.StringFixed(4),
	})
}

func parseRequiredCurrencyQuery(r *http.Request) (string, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("currency"))
	if raw == "" {
		return "", errors.New("currency query parameter is required")
	}
	c := strings.ToUpper(raw)
	if len(c) != 3 {
		return "", errors.New("currency must be a 3-letter ISO code")
	}
	for i := 0; i < 3; i++ {
		if c[i] < 'A' || c[i] > 'Z' {
			return "", errors.New("currency must be a 3-letter ISO code")
		}
	}
	return c, nil
}
