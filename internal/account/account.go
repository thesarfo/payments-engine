package account

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/thesarfo/payments-engine/pkg/money"
)

type AccountType string

const (
	AccountTypeAsset     AccountType = "ASSET"
	AccountTypeLiability AccountType = "LIABILITY"
	AccountTypeEquity    AccountType = "EQUITY"
	AccountTypeIncome    AccountType = "INCOME"
	AccountTypeExpense   AccountType = "EXPENSE"
)

type AccountStatus string

const (
	AccountStatusActive AccountStatus = "ACTIVE"
	AccountStatusFrozen AccountStatus = "FROZEN"
	AccountStatusClosed AccountStatus = "CLOSED"
)

type Account struct {
	ID       uuid.UUID
	Name     string
	Type     AccountType
	Currency money.Currency
	Balance  decimal.Decimal
	Status   AccountStatus
	Version  int64
}
