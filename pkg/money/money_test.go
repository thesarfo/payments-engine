package money

import (
	"strings"
	"testing"
)

func mustMoney(t *testing.T, amount string, currency Currency) Money {
	t.Helper()
	m, err := New(amount, currency)
	if err != nil {
		t.Fatalf("New(%q, %q): %v", amount, currency, err)
	}
	return m
}

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency Currency
		want     Money
		wantErr  bool
	}{
		{
			name:     "valid decimal",
			amount:   "10.50",
			currency: "GHS",
			want:     mustMoney(t, "10.50", "GHS"),
			wantErr:  false,
		},
		{
			name:     "valid integer string",
			amount:   "100",
			currency: "EUR",
			want:     mustMoney(t, "100", "EUR"),
			wantErr:  false,
		},
		{
			name:     "valid negative",
			amount:   "-0.01",
			currency: "GBP",
			want:     mustMoney(t, "-0.01", "GBP"),
			wantErr:  false,
		},
		{
			name:     "invalid amount",
			amount:   "not-a-number",
			currency: "GHS",
			want:     Money{},
			wantErr:  true,
		},
		{
			name:     "empty string invalid",
			amount:   "",
			currency: "GHS",
			want:     Money{},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.amount, tt.currency)
			if tt.wantErr {
				if err == nil {
					t.Fatal("New() succeeded, want error")
				}
				if !strings.Contains(err.Error(), "invalid amount") {
					t.Errorf("error = %v, want message containing 'invalid amount'", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMoney_Add(t *testing.T) {
	a := mustMoney(t, "10.00", "GHS")
	b := mustMoney(t, "5.50", "GHS")
	got, err := a.Add(b)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	want := mustMoney(t, "15.50", "GHS")
	if !got.Equal(want) {
		t.Errorf("Add() = %v, want %v", got, want)
	}
}

func TestMoney_Add_currencyMismatch(t *testing.T) {
	a := mustMoney(t, "10", "GHS")
	b := mustMoney(t, "5", "EUR")
	_, err := a.Add(b)
	if err == nil {
		t.Fatal("Add() expected currency mismatch error")
	}
	if !strings.Contains(err.Error(), "currency mismatch") {
		t.Errorf("error = %v, want currency mismatch", err)
	}
}

func TestMoney_Sub(t *testing.T) {
	a := mustMoney(t, "10.00", "GHS")
	b := mustMoney(t, "3.25", "GHS")
	got, err := a.Sub(b)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	want := mustMoney(t, "6.75", "GHS")
	if !got.Equal(want) {
		t.Errorf("Sub() = %v, want %v", got, want)
	}
}

func TestMoney_Sub_currencyMismatch(t *testing.T) {
	a := mustMoney(t, "10", "GHS")
	b := mustMoney(t, "5", "GBP")
	_, err := a.Sub(b)
	if err == nil {
		t.Fatal("Sub() expected currency mismatch error")
	}
	if !strings.Contains(err.Error(), "currency mismatch") {
		t.Errorf("error = %v, want currency mismatch", err)
	}
}

func TestMoney_IsZero(t *testing.T) {
	zero := mustMoney(t, "0", "GHS")
	if !zero.IsZero() {
		t.Error("IsZero() = false for 0")
	}
	nonZero := mustMoney(t, "0.01", "GHS")
	if nonZero.IsZero() {
		t.Error("IsZero() = true for 0.01")
	}
}

func TestMoney_IsPositive(t *testing.T) {
	pos := mustMoney(t, "1", "GHS")
	if !pos.IsPositive() {
		t.Error("IsPositive() = false for 1")
	}
	zero := mustMoney(t, "0", "GHS")
	if zero.IsPositive() {
		t.Error("IsPositive() = true for 0")
	}
	neg := mustMoney(t, "-1", "GHS")
	if neg.IsPositive() {
		t.Error("IsPositive() = true for -1")
	}
}

func TestMoney_Equal(t *testing.T) {
	a := mustMoney(t, "10.5", "GHS")
	b := mustMoney(t, "10.50", "GHS")
	if !a.Equal(b) {
		t.Error("Equal() = false for same amount normalized")
	}
	otherCurr := mustMoney(t, "10.5", "EUR")
	if a.Equal(otherCurr) {
		t.Error("Equal() = true for different currency")
	}
	otherAmt := mustMoney(t, "10.51", "GHS")
	if a.Equal(otherAmt) {
		t.Error("Equal() = true for different amount")
	}
}

func TestMoney_String(t *testing.T) {
	m := mustMoney(t, "1234.5", "GHS")
	if got := m.String(); got != "1234.50 GHS" {
		t.Errorf("String() = %q, want %q", got, "1234.50 GHS")
	}
}
