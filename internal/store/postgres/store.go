package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"stock-market/internal/domain"
	"stock-market/internal/model"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func Connect(dsn string) (*sql.DB, error) {
	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			lastErr = err
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			pingErr := db.PingContext(ctx)
			cancel()
			if pingErr == nil {
				return db, nil
			}
			lastErr = pingErr
			_ = db.Close()
		}
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("unable to connect to database after retries: %w", lastErr)
}

func InitSchema(ctx context.Context, db *sql.DB) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS wallets (
    id TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS bank_stocks (
    name TEXT PRIMARY KEY,
    quantity BIGINT NOT NULL CHECK (quantity >= 0)
);

CREATE TABLE IF NOT EXISTS wallet_stocks (
    wallet_id TEXT NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    stock_name TEXT NOT NULL REFERENCES bank_stocks(name),
    quantity BIGINT NOT NULL CHECK (quantity >= 0),
    PRIMARY KEY (wallet_id, stock_name)
);

CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    type TEXT NOT NULL CHECK (type IN ('buy', 'sell')),
    wallet_id TEXT NOT NULL,
    stock_name TEXT NOT NULL
);
`
	_, err := db.ExecContext(ctx, ddl)
	return err
}

func (s *Store) ListBankStocks(ctx context.Context) ([]model.StockItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, quantity FROM bank_stocks ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stocks := make([]model.StockItem, 0)
	for rows.Next() {
		var item model.StockItem
		if err := rows.Scan(&item.Name, &item.Quantity); err != nil {
			return nil, err
		}
		stocks = append(stocks, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stocks, nil
}

func (s *Store) SetBankStocks(ctx context.Context, stocks []model.StockItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE bank_stocks SET quantity = 0`); err != nil {
		return err
	}

	for _, stock := range stocks {
		_, err := tx.ExecContext(
			ctx,
			`INSERT INTO bank_stocks (name, quantity)
             VALUES ($1, $2)
             ON CONFLICT (name) DO UPDATE SET quantity = EXCLUDED.quantity`,
			stock.Name,
			stock.Quantity,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) WalletExists(ctx context.Context, walletID string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM wallets WHERE id = $1`, walletID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) ListWalletStocks(ctx context.Context, walletID string) ([]model.StockItem, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT stock_name, quantity
         FROM wallet_stocks
         WHERE wallet_id = $1 AND quantity > 0
         ORDER BY stock_name`,
		walletID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stocks := make([]model.StockItem, 0)
	for rows.Next() {
		var item model.StockItem
		if err := rows.Scan(&item.Name, &item.Quantity); err != nil {
			return nil, err
		}
		stocks = append(stocks, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stocks, nil
}

func (s *Store) StockExists(ctx context.Context, stockName string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM bank_stocks WHERE name = $1`, stockName).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) WalletStockQuantity(ctx context.Context, walletID, stockName string) (int64, error) {
	var quantity int64
	err := s.db.QueryRowContext(
		ctx,
		`SELECT quantity
         FROM wallet_stocks
         WHERE wallet_id = $1 AND stock_name = $2`,
		walletID,
		stockName,
	).Scan(&quantity)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return quantity, nil
}

func (s *Store) EnsureWallet(ctx context.Context, walletID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO wallets (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, walletID)
	return err
}

func (s *Store) ExecuteWalletOperation(ctx context.Context, walletID, stockName, opType string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var bankQty int64
	err = tx.QueryRowContext(
		ctx,
		`SELECT quantity FROM bank_stocks WHERE name = $1 FOR UPDATE`,
		stockName,
	).Scan(&bankQty)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrStockNotFound
	}
	if err != nil {
		return err
	}

	switch opType {
	case domain.OperationBuy:
		if bankQty < 1 {
			return domain.ErrNoBankStock
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bank_stocks SET quantity = quantity - 1 WHERE name = $1`, stockName); err != nil {
			return err
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO wallet_stocks (wallet_id, stock_name, quantity)
             VALUES ($1, $2, 1)
             ON CONFLICT (wallet_id, stock_name) DO UPDATE SET quantity = wallet_stocks.quantity + 1`,
			walletID,
			stockName,
		); err != nil {
			return err
		}
	case domain.OperationSell:
		var walletQty int64
		err := tx.QueryRowContext(
			ctx,
			`SELECT quantity
             FROM wallet_stocks
             WHERE wallet_id = $1 AND stock_name = $2
             FOR UPDATE`,
			walletID,
			stockName,
		).Scan(&walletQty)
		if errors.Is(err, sql.ErrNoRows) || walletQty < 1 {
			return domain.ErrNoWalletStock
		}
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(
			ctx,
			`UPDATE wallet_stocks
             SET quantity = quantity - 1
             WHERE wallet_id = $1 AND stock_name = $2`,
			walletID,
			stockName,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM wallet_stocks
             WHERE wallet_id = $1 AND stock_name = $2 AND quantity = 0`,
			walletID,
			stockName,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bank_stocks SET quantity = quantity + 1 WHERE name = $1`, stockName); err != nil {
			return err
		}
	default:
		return domain.ErrInvalidOperation
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO audit_log (type, wallet_id, stock_name) VALUES ($1, $2, $3)`,
		opType,
		walletID,
		stockName,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) ListLog(ctx context.Context) ([]model.LogEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT type, wallet_id, stock_name FROM audit_log ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logEntries := make([]model.LogEntry, 0)
	for rows.Next() {
		var entry model.LogEntry
		if err := rows.Scan(&entry.Type, &entry.WalletID, &entry.StockName); err != nil {
			return nil, err
		}
		logEntries = append(logEntries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logEntries, nil
}
