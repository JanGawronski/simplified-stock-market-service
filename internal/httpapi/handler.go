package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"stock-market/internal/domain"
	"stock-market/internal/model"
	"stock-market/internal/service"
)

type Handler struct {
	service    *service.Service
	instanceID string
}

func New(svc *service.Service, instanceID string) *Handler {
	return &Handler{
		service:    svc,
		instanceID: instanceID,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Instance-Id", h.instanceID)

	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	if r.URL.Path == "/stocks" {
		switch r.Method {
		case http.MethodGet:
			h.handleGetStocks(w, r)
		case http.MethodPost:
			h.handleSetStocks(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if r.URL.Path == "/log" && r.Method == http.MethodGet {
		h.handleGetLog(w, r)
		return
	}

	if r.URL.Path == "/chaos" && r.Method == http.MethodPost {
		h.handleChaos(w)
		return
	}

	parts := splitPath(r.URL.Path)
	if len(parts) >= 2 && parts[0] == "wallets" {
		walletID := parts[1]
		if walletID == "" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}

		if len(parts) == 2 && r.Method == http.MethodGet {
			h.handleGetWallet(w, r, walletID)
			return
		}

		if len(parts) == 4 && parts[2] == "stocks" {
			stockName := parts[3]
			if stockName == "" {
				writeError(w, http.StatusNotFound, "not found")
				return
			}
			if r.Method == http.MethodGet {
				h.handleGetWalletStock(w, r, walletID, stockName)
				return
			}
			if r.Method == http.MethodPost {
				h.handleWalletOperation(w, r, walletID, stockName)
				return
			}
		}
	}

	writeError(w, http.StatusNotFound, "not found")
}

func (h *Handler) handleGetStocks(w http.ResponseWriter, r *http.Request) {
	stocks, err := h.service.GetStocks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database query failed")
		return
	}
	writeJSON(w, http.StatusOK, model.BankResponse{Stocks: stocks})
}

func (h *Handler) handleSetStocks(w http.ResponseWriter, r *http.Request) {
	var req model.SetStocksRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.SetStocks(r.Context(), req); err != nil {
		if writeDomainError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "database update failed")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleGetWallet(w http.ResponseWriter, r *http.Request, walletID string) {
	wallet, err := h.service.GetWallet(r.Context(), walletID)
	if err != nil {
		if writeDomainError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "database query failed")
		return
	}
	writeJSON(w, http.StatusOK, wallet)
}

func (h *Handler) handleGetWalletStock(w http.ResponseWriter, r *http.Request, walletID, stockName string) {
	quantity, err := h.service.GetWalletStockQuantity(r.Context(), walletID, stockName)
	if err != nil {
		if writeDomainError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "database query failed")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(strconv.FormatInt(quantity, 10)))
}

func (h *Handler) handleWalletOperation(w http.ResponseWriter, r *http.Request, walletID, stockName string) {
	var req model.WalletOperationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.WalletOperation(r.Context(), walletID, stockName, req); err != nil {
		if writeDomainError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "database transaction failed")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleGetLog(w http.ResponseWriter, r *http.Request) {
	logEntries, err := h.service.GetLog(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database query failed")
		return
	}
	writeJSON(w, http.StatusOK, model.LogResponse{Log: logEntries})
}

func (h *Handler) handleChaos(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(1)
	}()
}

func writeDomainError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, domain.ErrWalletNotFound),
		errors.Is(err, domain.ErrStockNotFound):
		writeError(w, http.StatusNotFound, err.Error())
		return true
	case errors.Is(err, domain.ErrNoBankStock),
		errors.Is(err, domain.ErrNoWalletStock),
		errors.Is(err, domain.ErrInvalidOperation),
		errors.Is(err, domain.ErrInvalidStockPayload),
		errors.Is(err, domain.ErrDuplicateStockName):
		writeError(w, http.StatusBadRequest, err.Error())
		return true
	default:
		return false
	}
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("unexpected additional JSON input")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, model.ErrorResponse{Error: msg})
}

func splitPath(p string) []string {
	trimmed := strings.Trim(p, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
