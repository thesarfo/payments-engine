package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {
            "name": "Ledgr"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/accounts": {
            "post": {
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "accounts"
                ],
                "summary": "Create an account",
                "parameters": [
                    {
                        "description": "Account details",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/dto.CreateAccountRequest"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Created",
                        "schema": {
                            "$ref": "#/definitions/dto.AccountResponse"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/accounts/{id}": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "accounts"
                ],
                "summary": "Get account by ID",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Account UUID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dto.AccountResponse"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "Not Found",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/accounts/{id}/audit": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "accounts"
                ],
                "summary": "Account audit log",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Account UUID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "example": "2024-01-01T00:00:00Z",
                        "description": "RFC3339 start time (inclusive)",
                        "name": "from",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "example": "2024-12-31T23:59:59Z",
                        "description": "RFC3339 end time (inclusive)",
                        "name": "to",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/dto.AuditEventResponse"
                            }
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/accounts/{id}/entries": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "accounts"
                ],
                "summary": "List journal entries",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Account UUID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/dto.AccountEntryResponse"
                            }
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "Not Found",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/health/clearing": {
            "get": {
                "description": "A non-zero balance indicates a double-entry invariant violation — a transfer settled without fully clearing. Returns 200 when healthy, 503 otherwise.",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "health"
                ],
                "summary": "Clearing account health",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dto.ClearingHealthResponse"
                        }
                    },
                    "503": {
                        "description": "Service Unavailable",
                        "schema": {
                            "$ref": "#/definitions/dto.ClearingHealthResponse"
                        }
                    }
                }
            }
        },
        "/ledger/trial-balance": {
            "get": {
                "description": "Returns debits, credits, and net for every account. The balanced field is true when net_total is zero, confirming the double-entry invariant holds.",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "ledger"
                ],
                "summary": "Trial balance",
                "parameters": [
                    {
                        "type": "string",
                        "example": "USD",
                        "description": "ISO 4217 currency code",
                        "name": "currency",
                        "in": "query",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dto.TrialBalanceResponse"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/transactions/{id}/audit": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "transfers"
                ],
                "summary": "Transaction audit log",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Transfer UUID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "example": "2024-01-01T00:00:00Z",
                        "description": "RFC3339 start time (inclusive)",
                        "name": "from",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "example": "2024-12-31T23:59:59Z",
                        "description": "RFC3339 end time (inclusive)",
                        "name": "to",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/dto.AuditEventResponse"
                            }
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "Not Found",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/transfers": {
            "post": {
                "description": "Moves funds between two accounts using double-entry bookkeeping. The X-Idempotency-Key header is required — submitting the same key twice returns the original response without creating a duplicate transfer.",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "transfers"
                ],
                "summary": "Initiate a transfer",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Idempotency key (UUID v4 recommended)",
                        "name": "X-Idempotency-Key",
                        "in": "header",
                        "required": true
                    },
                    {
                        "description": "Transfer details",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/dto.CreateTransferRequest"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Created",
                        "schema": {
                            "$ref": "#/definitions/dto.TransactionResponse"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "Not Found",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "409": {
                        "description": "Transfer with this idempotency key is already in progress",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    }
                }
            }
        },
        "/transfers/{id}": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "transfers"
                ],
                "summary": "Get transfer by ID",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Transfer UUID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/dto.TransactionResponse"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "404": {
                        "description": "Not Found",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/dto.ErrorResponse"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "dto.AccountEntryResponse": {
            "type": "object",
            "properties": {
                "amount": {
                    "type": "string",
                    "example": "250.0000"
                },
                "entry_description": {
                    "type": "string",
                    "example": "Transfer settlement"
                },
                "entry_id": {
                    "type": "string",
                    "example": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
                },
                "entry_status": {
                    "type": "string",
                    "enum": [
                        "POSTED",
                        "REVERSED"
                    ],
                    "example": "POSTED"
                },
                "line_description": {
                    "type": "string",
                    "example": "Debit leg of transfer"
                },
                "line_id": {
                    "type": "string",
                    "example": "c3d4e5f6-a7b8-9012-cdef-123456789012"
                },
                "line_type": {
                    "type": "string",
                    "enum": [
                        "DEBIT",
                        "CREDIT"
                    ],
                    "example": "DEBIT"
                },
                "posted_at": {
                    "type": "string",
                    "example": "2024-01-15T10:30:00Z"
                },
                "reference": {
                    "type": "string",
                    "example": "b2c3d4e5-f6a7-8901-bcde-f12345678901"
                },
                "sequence": {
                    "type": "integer",
                    "example": 1
                }
            }
        },
        "dto.AccountResponse": {
            "type": "object",
            "properties": {
                "balance": {
                    "type": "string",
                    "example": "1000.0000"
                },
                "currency": {
                    "type": "string",
                    "example": "USD"
                },
                "id": {
                    "type": "string",
                    "example": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
                },
                "name": {
                    "type": "string",
                    "example": "Operating Account"
                },
                "status": {
                    "type": "string",
                    "enum": [
                        "ACTIVE",
                        "FROZEN",
                        "CLOSED"
                    ],
                    "example": "ACTIVE"
                },
                "type": {
                    "type": "string",
                    "example": "ASSET"
                },
                "version": {
                    "type": "integer",
                    "example": 1
                }
            }
        },
        "dto.AuditEventResponse": {
            "type": "object",
            "properties": {
                "actor": {
                    "type": "string",
                    "example": "payments-engine"
                },
                "entity_id": {
                    "type": "string",
                    "example": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
                },
                "entity_type": {
                    "type": "string",
                    "enum": [
                        "account",
                        "transaction"
                    ],
                    "example": "account"
                },
                "event_type": {
                    "type": "string",
                    "example": "account.created"
                },
                "id": {
                    "type": "integer",
                    "example": 1
                },
                "occurred_at": {
                    "type": "string",
                    "example": "2024-01-15T10:30:00.000000000Z"
                },
                "payload": {
                    "type": "object"
                }
            }
        },
        "dto.ClearingHealthResponse": {
            "type": "object",
            "properties": {
                "balance": {
                    "type": "string",
                    "example": "0.0000"
                },
                "clearing_account_code": {
                    "type": "string",
                    "example": "GL_LIAB_CLEARING"
                },
                "currency": {
                    "type": "string",
                    "example": "USD"
                },
                "reason": {
                    "type": "string",
                    "example": "clearing_account_balance_non_zero"
                },
                "status": {
                    "type": "string",
                    "enum": [
                        "OK",
                        "ALERT"
                    ],
                    "example": "OK"
                }
            }
        },
        "dto.CreateAccountRequest": {
            "type": "object",
            "properties": {
                "currency": {
                    "type": "string",
                    "example": "USD"
                },
                "name": {
                    "type": "string",
                    "example": "Operating Account"
                },
                "type": {
                    "type": "string",
                    "enum": [
                        "ASSET",
                        "LIABILITY",
                        "EQUITY",
                        "INCOME",
                        "EXPENSE"
                    ],
                    "example": "ASSET"
                }
            }
        },
        "dto.CreateTransferRequest": {
            "type": "object",
            "properties": {
                "amount": {
                    "type": "string",
                    "example": "250.00"
                },
                "currency": {
                    "type": "string",
                    "example": "USD"
                },
                "description": {
                    "type": "string",
                    "example": "Invoice #1042 payment"
                },
                "from_account_id": {
                    "type": "string",
                    "example": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
                },
                "rail": {
                    "type": "string",
                    "example": "INTERNAL"
                },
                "to_account_id": {
                    "type": "string",
                    "example": "b2c3d4e5-f6a7-8901-bcde-f12345678901"
                }
            }
        },
        "dto.ErrorResponse": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "string",
                    "example": "account not found"
                }
            }
        },
        "dto.TransactionResponse": {
            "type": "object",
            "properties": {
                "amount": {
                    "type": "string",
                    "example": "250.0000"
                },
                "created_at": {
                    "type": "string",
                    "example": "2024-01-15T10:30:00.000000000Z"
                },
                "currency": {
                    "type": "string",
                    "example": "USD"
                },
                "description": {
                    "type": "string",
                    "example": "Invoice #1042 payment"
                },
                "expires_at": {
                    "type": "string",
                    "example": "2024-01-16T10:30:00.000000000Z"
                },
                "external_ref": {
                    "type": "string",
                    "example": "ext-ref-001"
                },
                "failure_reason": {
                    "type": "string",
                    "example": "insufficient_funds"
                },
                "from_account_id": {
                    "type": "string",
                    "example": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
                },
                "id": {
                    "type": "string",
                    "example": "d4e5f6a7-b8c9-0123-defa-234567890123"
                },
                "idempotency_key": {
                    "type": "string",
                    "example": "550e8400-e29b-41d4-a716-446655440000"
                },
                "journal_entry_id": {
                    "type": "string",
                    "example": "e5f6a7b8-c9d0-1234-efab-345678901234"
                },
                "rail": {
                    "type": "string",
                    "example": "INTERNAL"
                },
                "settled_at": {
                    "type": "string",
                    "example": "2024-01-15T10:30:01.000000000Z"
                },
                "status": {
                    "type": "string",
                    "enum": [
                        "PENDING",
                        "PROCESSING",
                        "SETTLED",
                        "FAILED",
                        "REVERSED",
                        "ON_HOLD",
                        "EXPIRED",
                        "RECONCILED"
                    ],
                    "example": "SETTLED"
                },
                "to_account_id": {
                    "type": "string",
                    "example": "b2c3d4e5-f6a7-8901-bcde-f12345678901"
                },
                "updated_at": {
                    "type": "string",
                    "example": "2024-01-15T10:30:01.000000000Z"
                }
            }
        },
        "dto.TrialBalanceResponse": {
            "type": "object",
            "properties": {
                "balanced": {
                    "type": "boolean",
                    "example": true
                },
                "currency": {
                    "type": "string",
                    "example": "USD"
                },
                "net_total": {
                    "type": "string",
                    "example": "0.0000"
                },
                "rows": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/dto.TrialBalanceRow"
                    }
                }
            }
        },
        "dto.TrialBalanceRow": {
            "type": "object",
            "properties": {
                "account_id": {
                    "type": "string",
                    "example": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
                },
                "code": {
                    "type": "string",
                    "example": "GL_ASSET_CASH"
                },
                "name": {
                    "type": "string",
                    "example": "Cash"
                },
                "net": {
                    "type": "string",
                    "example": "2000.0000"
                },
                "total_credits": {
                    "type": "string",
                    "example": "3000.0000"
                },
                "total_debits": {
                    "type": "string",
                    "example": "5000.0000"
                }
            }
        }
    }
}`

var SwaggerInfo = &swag.Spec{
	Version:          "1.0.0",
	Host:             "localhost:8080",
	BasePath:         "/api/v1",
	Schemes:          []string{},
	Title:            "Ledgr — Payments Engine",
	Description:      "A double-entry ledger and transfer orchestration service for fintech infrastructure.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
