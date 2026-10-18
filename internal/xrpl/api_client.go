package xrpl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Kirk-V/XRPLTrading/internal/config"
	"github.com/Kirk-V/XRPLTrading/internal/domain"
	"github.com/gorilla/websocket"
)

type APIClient struct {
	config config.Config
	http   *http.Client
	ids    atomic.Uint64
}

func NewAPIClient(cfg config.Config) (*APIClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &APIClient{config: cfg, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (c *APIClient) Connect(ctx context.Context) error {
	return c.call(ctx, c.config.JSONRPC, "server_info", nil, nil)
}

func (c *APIClient) BookOffers(ctx context.Context, pair domain.Pair, limit int) ([]Offer, error) {
	if err := pair.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 400 {
		return nil, errors.New("book limit must be between 1 and 400")
	}
	result, err := c.bookAt(ctx, pair.Base, pair.Quote, "validated", limit)
	if err != nil {
		return nil, err
	}
	offers := make([]Offer, 0, len(result.Offers))
	for _, raw := range result.Offers {
		gets, err := amount(raw.Gets, pair.Base)
		if err != nil {
			return nil, err
		}
		pays, err := amount(raw.Pays, pair.Quote)
		if err != nil {
			return nil, err
		}
		offers = append(offers, Offer{Owner: raw.Account, Sequence: raw.Sequence, Gets: decimal(gets), Pays: decimal(pays)})
	}
	return offers, nil
}

func (c *APIClient) SubscribeBook(ctx context.Context, pair domain.Pair) (<-chan BookUpdate, error) {
	if err := pair.Validate(); err != nil {
		return nil, err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.config.WebSocket, nil)
	if err != nil {
		return nil, fmt.Errorf("dial XRPL websocket: %w", err)
	}
	request := map[string]any{
		"id":      c.ids.Add(1),
		"command": "subscribe",
		"books": []any{map[string]any{
			"taker_gets": currencyObject(pair.Base),
			"taker_pays": currencyObject(pair.Quote),
			"both":       true,
			"snapshot":   true,
		}},
	}
	if err := conn.WriteJSON(request); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("subscribe to XRPL book: %w", err)
	}
	updates := make(chan BookUpdate)
	go readBookUpdates(ctx, conn, updates)
	return updates, nil
}

func (c *APIClient) SubscribeLedger(ctx context.Context) (<-chan LedgerUpdate, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.config.WebSocket, nil)
	if err != nil {
		return nil, fmt.Errorf("dial XRPL websocket: %w", err)
	}
	request := map[string]any{
		"id":      c.ids.Add(1),
		"command": "subscribe",
		"streams": []string{"ledger"},
	}
	if err := conn.WriteJSON(request); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("subscribe to XRPL ledger: %w", err)
	}
	updates := make(chan LedgerUpdate)
	go readLedgerUpdates(ctx, conn, updates)
	return updates, nil
}

func currencyObject(asset domain.Asset) map[string]string {
	obj := map[string]string{"currency": asset.Currency}
	if !asset.IsXRP() {
		obj["issuer"] = asset.Issuer
	}
	return obj
}

type rpcRequest struct {
	ID      uint64           `json:"id"`
	Command string           `json:"method"`
	Params  []map[string]any `json:"params,omitempty"`
}

type rpcResponse struct {
	Status       string          `json:"status"`
	Type         string          `json:"type"`
	Error        string          `json:"error"`
	ErrorMessage string          `json:"error_message"`
	Result       json.RawMessage `json:"result"`
}

func (c *APIClient) call(ctx context.Context, endpoint, command string, params map[string]any, result any) error {
	args := map[string]any{"api_version": 2}
	for key, value := range params {
		args[key] = value
	}
	req := rpcRequest{ID: c.ids.Add(1), Command: command, Params: []map[string]any{args}}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("XRPL %s request: %w", command, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("XRPL %s returned HTTP %s", command, resp.Status)
	}
	var decoded rpcResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&decoded); err != nil {
		return fmt.Errorf("decode XRPL %s response: %w", command, err)
	}
	if decoded.Error != "" {
		return fmt.Errorf("XRPL %s failed: %s", command, decoded.ErrorMessageOrCode())
	}
	if len(decoded.Result) == 0 || string(decoded.Result) == "null" {
		return fmt.Errorf("XRPL %s returned no result", command)
	}
	var status rpcResponse
	if err := json.Unmarshal(decoded.Result, &status); err != nil {
		return fmt.Errorf("decode XRPL %s result: %w", command, err)
	}
	if status.Error != "" || status.Status == "error" {
		return fmt.Errorf("XRPL %s failed: %s", command, status.ErrorMessageOrCode())
	}
	if result != nil && len(decoded.Result) > 0 {
		if err := json.Unmarshal(decoded.Result, result); err != nil {
			return fmt.Errorf("decode XRPL %s result: %w", command, err)
		}
	}
	return nil
}

func (r rpcResponse) ErrorMessageOrCode() string {
	if r.ErrorMessage != "" {
		return r.ErrorMessage
	}
	return r.Error
}

func readBookUpdates(ctx context.Context, conn *websocket.Conn, output chan<- BookUpdate) {
	defer close(output)
	defer conn.Close()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	for {
		var message struct {
			Type        string  `json:"type"`
			Status      string  `json:"status"`
			Error       string  `json:"error"`
			LedgerIndex uint32  `json:"ledger_index"`
			Offers      []Offer `json:"offers"`
		}
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		if message.Error != "" || message.Type == "response" || len(message.Offers) == 0 {
			continue
		}
		select {
		case output <- BookUpdate{LedgerIndex: message.LedgerIndex, Offers: message.Offers}:
		case <-ctx.Done():
			return
		}
	}
}

func readLedgerUpdates(ctx context.Context, conn *websocket.Conn, output chan<- LedgerUpdate) {
	defer close(output)
	defer conn.Close()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	for {
		var message struct {
			Type        string `json:"type"`
			Error       string `json:"error"`
			LedgerIndex uint32 `json:"ledger_index"`
			LedgerHash  string `json:"ledger_hash"`
			LedgerTime  uint32 `json:"ledger_time"`
			TxnCount    uint32 `json:"txn_count"`
		}
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		if message.Error != "" || message.Type != "ledgerClosed" || message.LedgerIndex == 0 {
			continue
		}
		select {
		case output <- LedgerUpdate{
			LedgerIndex: message.LedgerIndex,
			LedgerHash:  message.LedgerHash,
			LedgerTime:  message.LedgerTime,
			TxnCount:    message.TxnCount,
		}:
		case <-ctx.Done():
			return
		}
	}
}
