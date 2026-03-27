package money

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type Currency string

type Money struct {
	amount   decimal.Decimal
	currency Currency
}

func New(amount string, currency Currency) (Money, error) {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		return Money{}, fmt.Errorf("invalid amount: %q: %w", amount, err)
	}
	return Money{amount: d, currency: currency}, nil
}

func FromDecimal(amount decimal.Decimal, currency Currency) Money {
	return Money{amount: amount, currency: currency}
}

func (m Money) Amount() decimal.Decimal { return m.amount }

func (m Money) Currency() Currency { return m.currency }

func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("currency mismatch: %s vs %s", m.currency, other.currency)
	}
	return Money{amount: m.amount.Add(other.amount), currency: m.currency}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("currency mismatch: %s vs %s", m.currency, other.currency)
	}
	result := m.amount.Sub(other.amount)
	return Money{amount: result, currency: m.currency}, nil
}

func (m Money) IsZero() bool { return m.amount.IsZero() }

func (m Money) IsPositive() bool { return m.amount.IsPositive() }

func (m Money) IsNegative() bool { return m.amount.IsNegative() }

func (m Money) SameCurrency(other Money) bool { return m.currency == other.currency }

func (m Money) Cmp(other Money) (int, error) {
	if !m.SameCurrency(other) {
		return 0, fmt.Errorf("currency mismatch: %s vs %s", m.currency, other.currency)
	}
	return m.amount.Cmp(other.amount), nil
}

func (m Money) LessThan(other Money) (bool, error) {
	cmp, err := m.Cmp(other)
	if err != nil {
		return false, err
	}
	return cmp < 0, nil
}

func (m Money) GreaterThan(other Money) (bool, error) {
	cmp, err := m.Cmp(other)
	if err != nil {
		return false, err
	}
	return cmp > 0, nil
}

func (m Money) Equal(other Money) bool {
	return m.currency == other.currency && m.amount.Equal(other.amount)
}

func (m Money) String() string {
	return fmt.Sprintf("%s %s", m.amount.StringFixed(2), m.currency)
}
