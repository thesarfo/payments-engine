package dto

type TrialBalanceRow struct {
	AccountID    string `json:"account_id"    example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	Code         string `json:"code"          example:"GL_ASSET_CASH"`
	Name         string `json:"name"          example:"Cash"`
	TotalDebits  string `json:"total_debits"  example:"5000.0000"`
	TotalCredits string `json:"total_credits" example:"3000.0000"`
	Net          string `json:"net"           example:"2000.0000"`
}

type TrialBalanceResponse struct {
	Currency string            `json:"currency"  example:"USD"`
	Rows     []TrialBalanceRow `json:"rows"`
	Balanced bool              `json:"balanced"  example:"true"`
	NetTotal string            `json:"net_total" example:"0.0000"`
}
