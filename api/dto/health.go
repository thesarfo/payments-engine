package dto

type ClearingHealthResponse struct {
	Status              string `json:"status"                example:"OK"               enums:"OK,ALERT"`
	ClearingAccountCode string `json:"clearing_account_code" example:"GL_LIAB_CLEARING"`
	Currency            string `json:"currency,omitempty"    example:"USD"`
	Balance             string `json:"balance"               example:"0.0000"`
	Reason              string `json:"reason,omitempty"      example:"clearing_account_balance_non_zero"`
}
