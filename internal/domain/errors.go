package domain

import "errors"

const (
	OperationBuy  = "buy"
	OperationSell = "sell"
)

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrStockNotFound       = errors.New("stock not found")
	ErrNoBankStock         = errors.New("no stock available in bank")
	ErrNoWalletStock       = errors.New("no stock available in wallet")
	ErrInvalidOperation    = errors.New("type must be buy or sell")
	ErrInvalidStockPayload = errors.New("invalid stock payload")
	ErrDuplicateStockName  = errors.New("duplicate stock name")
)
