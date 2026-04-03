package dto

type TrialBalanceRow struct {
	AccountID    string `json:"account_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	TotalDebits  string `json:"total_debits"`
	TotalCredits string `json:"total_credits"`
	Net          string `json:"net"`
}

type TrialBalanceResponse struct {
	Currency string            `json:"currency"`
	Rows     []TrialBalanceRow `json:"rows"`
	Balanced bool              `json:"balanced"`
	NetTotal string            `json:"net_total"`
}
