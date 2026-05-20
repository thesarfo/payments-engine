package dto

type CreateAccountRequest struct {
	Name     string `json:"name"     example:"Operating Account"`
	Type     string `json:"type"     example:"ASSET"            enums:"ASSET,LIABILITY,EQUITY,INCOME,EXPENSE"`
	Currency string `json:"currency" example:"USD"`
}

type AccountResponse struct {
	ID       string `json:"id"       example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	Name     string `json:"name"     example:"Operating Account"`
	Type     string `json:"type"     example:"ASSET"`
	Currency string `json:"currency" example:"USD"`
	Balance  string `json:"balance"  example:"1000.0000"`
	Status   string `json:"status"   example:"ACTIVE"           enums:"ACTIVE,FROZEN,CLOSED"`
	Version  int64  `json:"version"  example:"1"`
}

type AccountEntryResponse struct {
	EntryID          string `json:"entry_id"          example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	PostedAt         string `json:"posted_at"         example:"2024-01-15T10:30:00Z"`
	EntryDescription string `json:"entry_description" example:"Transfer settlement"`
	Reference        string `json:"reference"         example:"b2c3d4e5-f6a7-8901-bcde-f12345678901"`
	EntryStatus      string `json:"entry_status"      example:"POSTED"                                 enums:"POSTED,REVERSED"`
	LineID           string `json:"line_id"           example:"c3d4e5f6-a7b8-9012-cdef-123456789012"`
	LineType         string `json:"line_type"         example:"DEBIT"                                  enums:"DEBIT,CREDIT"`
	Amount           string `json:"amount"            example:"250.0000"`
	LineDescription  string `json:"line_description"  example:"Debit leg of transfer"`
	Sequence         int16  `json:"sequence"          example:"1"`
}

type UpdateAccountStatusRequest struct {
	Status string `json:"status" example:"FROZEN" enums:"ACTIVE,FROZEN,CLOSED"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"account not found"`
}
