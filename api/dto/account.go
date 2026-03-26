package dto

type CreateAccountRequest struct {
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

type AccountEntryResponse struct {
	EntryID          string `json:"entry_id"`
	PostedAt         string `json:"posted_at"`
	EntryDescription string `json:"entry_description"`
	Reference        string `json:"reference"`
	EntryStatus      string `json:"entry_status"`
	LineID           string `json:"line_id"`
	LineType         string `json:"line_type"`
	Amount           string `json:"amount"`
	LineDescription  string `json:"line_description"`
	Sequence         int16  `json:"sequence"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
