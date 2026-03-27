package account

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/pkg/money"
)

type AccountService struct {
	repo Repository
}

type CreateAccountInput struct {
	Name     string
	Type     AccountType
	Currency money.Currency
}

func NewAccountService(repo Repository) *AccountService {
	return &AccountService{repo: repo}
}

func (s *AccountService) CreateAccount(ctx context.Context, input CreateAccountInput) (Account, error) {
	name := strings.TrimSpace(input.Name)

	if name == "" {
		return Account{}, fmt.Errorf("name is required")
	}
	if !isValidAccountType(input.Type) {
		return Account{}, fmt.Errorf("invalid account type")
	}

	if !isValidISO4217Currency(input.Currency) {
		return Account{}, fmt.Errorf("invalid currency code")
	}

	acc := Account{
		Name:     name,
		Type:     input.Type,
		Currency: input.Currency,
		Balance:  decimal.Zero,
		Status:   AccountStatusActive,
	}

	return s.repo.CreateAccount(ctx, acc)
}

func (s *AccountService) GetAccountByID(ctx context.Context, id uuid.UUID) (Account, error) {
	return s.repo.GetAccountByID(ctx, id)
}

func isValidISO4217Currency(currency money.Currency) bool {
	code := string(currency)

	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	switch code {
	case "USD", "EUR", "GBP", "NGN", "KES", "ZAR", "GHS", "JPY", "CAD", "AUD":
		return true
	default:
		return false
	}
}

func isValidAccountType(t AccountType) bool {
	switch t {
	case AccountTypeAsset, AccountTypeLiability, AccountTypeEquity, AccountTypeIncome, AccountTypeExpense:
		return true
	default:
		return false
	}
}
