package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"stock-market/internal/domain"
	"stock-market/internal/model"
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

func TestSetStocksValidation(t *testing.T) {
	tests := []struct {
		name string
		req  model.SetStocksRequest
		want error
	}{
		{
			name: "empty stock name",
			req: model.SetStocksRequest{
				Stocks: []model.StockItem{{Name: "", Quantity: 1}},
			},
			want: domain.ErrInvalidStockPayload,
		},
		{
			name: "negative quantity",
			req: model.SetStocksRequest{
				Stocks: []model.StockItem{{Name: "stock1", Quantity: -1}},
			},
			want: domain.ErrInvalidStockPayload,
		},
		{
			name: "duplicate names",
			req: model.SetStocksRequest{
				Stocks: []model.StockItem{
					{Name: "stock1", Quantity: 1},
					{Name: "stock1", Quantity: 2},
				},
			},
			want: domain.ErrDuplicateStockName,
		},
	}

	svc := New(&repoMock{})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.SetStocks(context.Background(), tc.req)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected error %v, got %v", tc.want, err)
			}
		})
	}
}

func TestSetStocksDelegatesOnValidPayload(t *testing.T) {
	var got []model.StockItem
	repo := &repoMock{
		setBankStocksFn: func(_ context.Context, stocks []model.StockItem) error {
			got = append(got, stocks...)
			return nil
		},
	}

	svc := New(repo)
	want := []model.StockItem{
		{Name: "stock1", Quantity: 2},
		{Name: "stock2", Quantity: 3},
	}
	if err := svc.SetStocks(context.Background(), model.SetStocksRequest{Stocks: want}); err != nil {
		t.Fatalf("SetStocks returned error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("setBankStocks called with %+v, want %+v", got, want)
	}
}

func TestGetWalletNotFound(t *testing.T) {
	repo := &repoMock{
		walletExistsFn: func(context.Context, string) (bool, error) { return false, nil },
	}
	svc := New(repo)

	_, err := svc.GetWallet(context.Background(), "w1")
	if !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("expected ErrWalletNotFound, got %v", err)
	}
}

func TestGetWalletStockQuantityChecksStockFirst(t *testing.T) {
	walletExistsCalled := false
	repo := &repoMock{
		stockExistsFn:  func(context.Context, string) (bool, error) { return false, nil },
		walletExistsFn: func(context.Context, string) (bool, error) { walletExistsCalled = true; return true, nil },
	}
	svc := New(repo)

	_, err := svc.GetWalletStockQuantity(context.Background(), "w1", "stock1")
	if !errors.Is(err, domain.ErrStockNotFound) {
		t.Fatalf("expected ErrStockNotFound, got %v", err)
	}
	if walletExistsCalled {
		t.Fatal("wallet existence check should not run when stock does not exist")
	}
}

func TestWalletOperationCreatesWalletThenExecutes(t *testing.T) {
	var callOrder []string
	repo := &repoMock{
		ensureWalletFn: func(_ context.Context, walletID string) error {
			callOrder = append(callOrder, "ensure:"+walletID)
			return nil
		},
		executeWalletOperationFn: func(_ context.Context, walletID, stockName, opType string) error {
			callOrder = append(callOrder, "execute:"+walletID+":"+stockName+":"+opType)
			return nil
		},
	}
	svc := New(repo)

	err := svc.WalletOperation(
		context.Background(),
		"wallet-1",
		"stock-1",
		model.WalletOperationRequest{Type: domain.OperationBuy},
	)
	if err != nil {
		t.Fatalf("WalletOperation returned error: %v", err)
	}

	wantOrder := []string{"ensure:wallet-1", "execute:wallet-1:stock-1:buy"}
	if !reflect.DeepEqual(callOrder, wantOrder) {
		t.Fatalf("call order = %#v, want %#v", callOrder, wantOrder)
	}
}

func TestWalletOperationRejectsInvalidType(t *testing.T) {
	ensureCalled := false
	repo := &repoMock{
		ensureWalletFn: func(context.Context, string) error {
			ensureCalled = true
			return nil
		},
	}
	svc := New(repo)

	err := svc.WalletOperation(context.Background(), "wallet-1", "stock-1", model.WalletOperationRequest{Type: "hold"})
	if !errors.Is(err, domain.ErrInvalidOperation) {
		t.Fatalf("expected ErrInvalidOperation, got %v", err)
	}
	if ensureCalled {
		t.Fatal("EnsureWallet should not be called for invalid operation type")
	}
}
