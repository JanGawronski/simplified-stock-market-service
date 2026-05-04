package service

import (
	"context"
	"strings"

	"stock-market/internal/domain"
	"stock-market/internal/model"
)

type Repository interface {
	ListBankStocks(ctx context.Context) ([]model.StockItem, error)
	SetBankStocks(ctx context.Context, stocks []model.StockItem) error
	WalletExists(ctx context.Context, walletID string) (bool, error)
	ListWalletStocks(ctx context.Context, walletID string) ([]model.StockItem, error)
	StockExists(ctx context.Context, stockName string) (bool, error)
	WalletStockQuantity(ctx context.Context, walletID, stockName string) (int64, error)
	EnsureWallet(ctx context.Context, walletID string) error
	ExecuteWalletOperation(ctx context.Context, walletID, stockName, opType string) error
	ListLog(ctx context.Context) ([]model.LogEntry, error)
}

type Service struct {
	repo Repository
}

func New(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetStocks(ctx context.Context) ([]model.StockItem, error) {
	return s.repo.ListBankStocks(ctx)
}

func (s *Service) SetStocks(ctx context.Context, req model.SetStocksRequest) error {
	seen := make(map[string]struct{}, len(req.Stocks))
	for _, stock := range req.Stocks {
		if strings.TrimSpace(stock.Name) == "" || stock.Quantity < 0 {
			return domain.ErrInvalidStockPayload
		}
		if _, ok := seen[stock.Name]; ok {
			return domain.ErrDuplicateStockName
		}
		seen[stock.Name] = struct{}{}
	}
	return s.repo.SetBankStocks(ctx, req.Stocks)
}

func (s *Service) GetWallet(ctx context.Context, walletID string) (model.WalletResponse, error) {
	exists, err := s.repo.WalletExists(ctx, walletID)
	if err != nil {
		return model.WalletResponse{}, err
	}
	if !exists {
		return model.WalletResponse{}, domain.ErrWalletNotFound
	}

	stocks, err := s.repo.ListWalletStocks(ctx, walletID)
	if err != nil {
		return model.WalletResponse{}, err
	}

	return model.WalletResponse{
		ID:     walletID,
		Stocks: stocks,
	}, nil
}

func (s *Service) GetWalletStockQuantity(ctx context.Context, walletID, stockName string) (int64, error) {
	stockExists, err := s.repo.StockExists(ctx, stockName)
	if err != nil {
		return 0, err
	}
	if !stockExists {
		return 0, domain.ErrStockNotFound
	}

	walletExists, err := s.repo.WalletExists(ctx, walletID)
	if err != nil {
		return 0, err
	}
	if !walletExists {
		return 0, domain.ErrWalletNotFound
	}

	return s.repo.WalletStockQuantity(ctx, walletID, stockName)
}

func (s *Service) WalletOperation(ctx context.Context, walletID, stockName string, req model.WalletOperationRequest) error {
	if req.Type != domain.OperationBuy && req.Type != domain.OperationSell {
		return domain.ErrInvalidOperation
	}

	if err := s.repo.EnsureWallet(ctx, walletID); err != nil {
		return err
	}

	return s.repo.ExecuteWalletOperation(ctx, walletID, stockName, req.Type)
}

func (s *Service) GetLog(ctx context.Context) ([]model.LogEntry, error) {
	return s.repo.ListLog(ctx)
}
