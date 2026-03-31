package dto

type ClearingHealthResponse struct {
	Status              string `json:"status"` // OK or ALERT
	ClearingAccountCode string `json:"clearing_account_code"`
	Currency            string `json:"currency,omitempty"`
	Balance             string `json:"balance"`
	Reason              string `json:"reason,omitempty"`
}
