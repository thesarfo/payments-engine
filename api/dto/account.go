package dto

type CreateAccountRequest struct{
	Name     string `json:"name"`
	Type     string `json:"type"`
	Currency string `json:"currency"`
}

type AccountResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Currency string `json:"currency"`
	Balance  string `json:"balance"`
	Status   string `json:"status"`
	Version  int64  `json:"version"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}