package model

type StockItem struct {
	Name     string `json:"name"`
	Quantity int64  `json:"quantity"`
}

type WalletResponse struct {
	ID     string      `json:"id"`
	Stocks []StockItem `json:"stocks"`
}

type BankResponse struct {
	Stocks []StockItem `json:"stocks"`
}

type SetStocksRequest struct {
	Stocks []StockItem `json:"stocks"`
}

type WalletOperationRequest struct {
	Type string `json:"type"`
}

type LogEntry struct {
	Type      string `json:"type"`
	WalletID  string `json:"wallet_id"`
	StockName string `json:"stock_name"`
}

type LogResponse struct {
	Log []LogEntry `json:"log"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
