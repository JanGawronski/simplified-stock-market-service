package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stock-market/internal/domain"
	"stock-market/internal/model"
	"stock-market/internal/service"
)

type repoMock struct {
	listBankStocksFn         func(context.Context) ([]model.StockItem, error)
	setBankStocksFn          func(context.Context, []model.StockItem) error
	walletExistsFn           func(context.Context, string) (bool, error)
	listWalletStocksFn       func(context.Context, string) ([]model.StockItem, error)
	stockExistsFn            func(context.Context, string) (bool, error)
	walletStockQuantityFn    func(context.Context, string, string) (int64, error)
	ensureWalletFn           func(context.Context, string) error
	executeWalletOperationFn func(context.Context, string, string, string) error
	listLogFn                func(context.Context) ([]model.LogEntry, error)
}

func (m *repoMock) ListBankStocks(ctx context.Context) ([]model.StockItem, error) {
	if m.listBankStocksFn != nil {
		return m.listBankStocksFn(ctx)
	}
	return nil, nil
}

func (m *repoMock) SetBankStocks(ctx context.Context, stocks []model.StockItem) error {
	if m.setBankStocksFn != nil {
		return m.setBankStocksFn(ctx, stocks)
	}
	return nil
}

func (m *repoMock) WalletExists(ctx context.Context, walletID string) (bool, error) {
	if m.walletExistsFn != nil {
		return m.walletExistsFn(ctx, walletID)
	}
	return false, nil
}

func (m *repoMock) ListWalletStocks(ctx context.Context, walletID string) ([]model.StockItem, error) {
	if m.listWalletStocksFn != nil {
		return m.listWalletStocksFn(ctx, walletID)
	}
	return nil, nil
}

func (m *repoMock) StockExists(ctx context.Context, stockName string) (bool, error) {
	if m.stockExistsFn != nil {
		return m.stockExistsFn(ctx, stockName)
	}
	return false, nil
}

func (m *repoMock) WalletStockQuantity(ctx context.Context, walletID, stockName string) (int64, error) {
	if m.walletStockQuantityFn != nil {
		return m.walletStockQuantityFn(ctx, walletID, stockName)
	}
	return 0, nil
}

func (m *repoMock) EnsureWallet(ctx context.Context, walletID string) error {
	if m.ensureWalletFn != nil {
		return m.ensureWalletFn(ctx, walletID)
	}
	return nil
}

func (m *repoMock) ExecuteWalletOperation(ctx context.Context, walletID, stockName, opType string) error {
	if m.executeWalletOperationFn != nil {
		return m.executeWalletOperationFn(ctx, walletID, stockName, opType)
	}
	return nil
}

func (m *repoMock) ListLog(ctx context.Context) ([]model.LogEntry, error) {
	if m.listLogFn != nil {
		return m.listLogFn(ctx)
	}
	return nil, nil
}

func TestGetStocksReturnsExpectedResponse(t *testing.T) {
	repo := &repoMock{
		listBankStocksFn: func(context.Context) ([]model.StockItem, error) {
			return []model.StockItem{
				{Name: "stock1", Quantity: 2},
				{Name: "stock2", Quantity: 1},
			}, nil
		},
	}

	h := New(service.New(repo), "test-instance")
	req := httptest.NewRequest(http.MethodGet, "/stocks", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("X-Instance-Id") != "test-instance" {
		t.Fatalf("X-Instance-Id = %q, want %q", rr.Header().Get("X-Instance-Id"), "test-instance")
	}

	var resp model.BankResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Stocks) != 2 {
		t.Fatalf("stocks length = %d, want 2", len(resp.Stocks))
	}
}

func TestPostWalletOperationMapsStockNotFoundTo404(t *testing.T) {
	repo := &repoMock{
		ensureWalletFn: func(context.Context, string) error { return nil },
		executeWalletOperationFn: func(context.Context, string, string, string) error {
			return domain.ErrStockNotFound
		},
	}

	h := New(service.New(repo), "test-instance")
	req := httptest.NewRequest(
		http.MethodPost,
		"/wallets/w1/stocks/stock1",
		bytes.NewBufferString(`{"type":"buy"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetWalletStockReturnsPlainNumber(t *testing.T) {
	repo := &repoMock{
		stockExistsFn:         func(context.Context, string) (bool, error) { return true, nil },
		walletExistsFn:        func(context.Context, string) (bool, error) { return true, nil },
		walletStockQuantityFn: func(context.Context, string, string) (int64, error) { return 7, nil },
	}

	h := New(service.New(repo), "test-instance")
	req := httptest.NewRequest(http.MethodGet, "/wallets/w1/stocks/stock1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("content-type = %q, want text/plain", rr.Header().Get("Content-Type"))
	}
	if strings.TrimSpace(rr.Body.String()) != "7" {
		t.Fatalf("body = %q, want %q", rr.Body.String(), "7")
	}
}

func TestPostStocksInvalidJSONReturns400(t *testing.T) {
	h := New(service.New(&repoMock{}), "test-instance")
	req := httptest.NewRequest(
		http.MethodPost,
		"/stocks",
		bytes.NewBufferString(`{"stocks":[{"name":"stock1","quantity":1}],"extra":1}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestUnknownRouteReturns404(t *testing.T) {
	h := New(service.New(&repoMock{}), "test-instance")
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
