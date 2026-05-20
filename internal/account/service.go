package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/thesarfo/payments-engine/internal/audit"
	"github.com/thesarfo/payments-engine/pkg/money"
)

var ErrInvalidStatusTransition = errors.New("invalid account status transition")

type AccountService struct {
	repo        Repository
	auditLogger audit.Logger
}

type CreateAccountInput struct {
	Name     string
	Type     AccountType
	Currency money.Currency
}

func NewAccountService(repo Repository) *AccountService {
	return &AccountService{repo: repo}
}

func (s *AccountService) WithAuditLogger(al audit.Logger) *AccountService {
	s.auditLogger = al
	return s
}

func (s *AccountService) logAudit(ctx context.Context, entityID uuid.UUID, eventType string, payload any) {
	if s.auditLogger == nil {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = s.auditLogger.Log(ctx, audit.AuditEvent{
		EntityType: audit.EntityAccount,
		EntityID:   entityID,
		EventType:  eventType,
		Actor:      "account-service",
		Payload:    b,
	})
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

	created, err := s.repo.CreateAccount(ctx, acc)
	if err != nil {
		return Account{}, err
	}
	s.logAudit(ctx, created.ID, audit.EventAccountCreated, map[string]any{
		"account_id": created.ID.String(),
		"name":       created.Name,
		"type":       string(created.Type),
		"currency":   string(created.Currency),
		"status":     string(created.Status),
	})
	return created, nil
}

func (s *AccountService) GetAccountByID(ctx context.Context, id uuid.UUID) (Account, error) {
	return s.repo.GetAccountByID(ctx, id)
}

func (s *AccountService) UpdateAccountStatus(ctx context.Context, id uuid.UUID, newStatus AccountStatus) (Account, error) {
	current, err := s.repo.GetAccountByID(ctx, id)
	if err != nil {
		return Account{}, err
	}

	if current.Status == newStatus {
		return Account{}, fmt.Errorf("%w: account is already %s", ErrInvalidStatusTransition, newStatus)
	}
	if current.Status == AccountStatusClosed {
		return Account{}, fmt.Errorf("%w: closed accounts cannot be transitioned", ErrInvalidStatusTransition)
	}
	if current.Status == AccountStatusFrozen && newStatus != AccountStatusActive && newStatus != AccountStatusClosed {
		return Account{}, fmt.Errorf("%w: frozen accounts can only be unfrozen or closed", ErrInvalidStatusTransition)
	}

	updated, err := s.repo.UpdateAccountStatus(ctx, id, newStatus)
	if err != nil {
		return Account{}, err
	}

	var eventType string
	switch newStatus {
	case AccountStatusFrozen:
		eventType = audit.EventAccountFrozen
	case AccountStatusClosed:
		eventType = audit.EventAccountClosed
	default:
		eventType = audit.EventAccountUnfrozen
	}
	s.logAudit(ctx, id, eventType, map[string]any{
		"from": string(current.Status),
		"to":   string(newStatus),
	})

	return updated, nil
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
