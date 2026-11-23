package evalcase

import "errors"

var (
	ErrInvalidAmount     = errors.New("amount must be positive")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

type Account struct {
	Balance int64 // cents
}

// Withdraw subtracts cents from the balance.
func (a *Account) Withdraw(cents int64) error {
	if cents <= 0 {
		return ErrInvalidAmount
	}
	if cents > a.Balance {
		return ErrInsufficientFunds
	}
	a.Balance -= cents
	return nil
}
