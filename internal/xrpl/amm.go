package xrpl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/Kirk-V/XRPLTrading/api/v1"
	"github.com/Kirk-V/XRPLTrading/internal/domain"
	"github.com/gorilla/websocket"
)

// AMMStreamUpdate is either a subscription acknowledgement, a ledger heartbeat,
// or a successful validated XRP/token AMM creation. Other transactions are ignored.
type AMMStreamUpdate struct {
	LedgerIndex uint32
	Creation    *v1.AMMCreation
}

type ammTransaction struct {
	Type    string          `json:"TransactionType"`
	Amount  json.RawMessage `json:"Amount"`
	Amount2 json.RawMessage `json:"Amount2"`
	Hash    string          `json:"hash"`
}

type ammMessage struct {
	ID                uint64         `json:"id"`
	Type              string         `json:"type"`
	Status            string         `json:"status"`
	Error             string         `json:"error"`
	Validated         bool           `json:"validated"`
	LedgerIndex       uint32         `json:"ledger_index"`
	Hash              string         `json:"hash"`
	Transaction       ammTransaction `json:"tx_json"`
	LegacyTransaction ammTransaction `json:"transaction"`
	Meta              struct {
		Result string `json:"TransactionResult"`
	} `json:"meta"`
}

func creationFromMessage(message ammMessage) (*v1.AMMCreation, error) {
	tx := message.Transaction
	if tx.Type == "" {
		tx = message.LegacyTransaction
	}
	if message.Type != "transaction" || !message.Validated || message.Meta.Result != "tesSUCCESS" || tx.Type != "AMMCreate" {
		return nil, nil
	}
	first, err := assetFromAMMAmount(tx.Amount)
	if err != nil {
		return nil, err
	}
	second, err := assetFromAMMAmount(tx.Amount2)
	if err != nil {
		return nil, err
	}
	// The application currently tracks XRP/issued-token markets only.
	if first.IsXRP() == second.IsXRP() {
		return nil, nil
	}
	token := first
	if first.IsXRP() {
		token = second
	}
	pair, err := ValidateMarketInput(token.Currency, token.Issuer)
	if err != nil {
		return nil, err
	}
	hash := message.Hash
	if hash == "" {
		hash = tx.Hash
	}
	if message.LedgerIndex == 0 || len(hash) != 64 {
		return nil, fmt.Errorf("AMM creation missing ledger or transaction hash")
	}
	return &v1.AMMCreation{Pair: pair, LedgerIndex: message.LedgerIndex, TransactionHash: hash, ObservedAt: time.Now().UTC()}, nil
}

func assetFromAMMAmount(raw json.RawMessage) (domain.Asset, error) {
	var drops string
	if json.Unmarshal(raw, &drops) == nil {
		n, err := amount(raw, domain.XRP())
		if err != nil {
			return domain.Asset{}, err
		}
		if n.Sign() <= 0 {
			return domain.Asset{}, fmt.Errorf("AMM initial XRP amount must be positive")
		}
		return domain.XRP(), nil
	}
	var asset domain.Asset
	if err := json.Unmarshal(raw, &asset); err != nil {
		return asset, fmt.Errorf("decode AMM asset: %w", err)
	}
	if err := asset.Validate(); err != nil {
		return asset, err
	}
	n, err := amount(raw, asset)
	if err != nil {
		return asset, err
	}
	if n.Sign() <= 0 {
		return asset, fmt.Errorf("AMM initial token amount must be positive")
	}
	return asset, nil
}

// WatchAMMs runs one subscription until cancellation, rejection, timeout, or
// disconnect. The application owns reconnects and reports gaps to the UI.
func (c *APIClient) WatchAMMs(ctx context.Context, emit func(AMMStreamUpdate)) error {
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second
	conn, _, err := dialer.DialContext(ctx, c.config.WebSocket, nil)
	if err != nil {
		return fmt.Errorf("connect AMM discovery: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	conn.SetReadLimit(8 << 20)
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	id := c.ids.Add(1)
	if err := conn.WriteJSON(map[string]any{"id": id, "command": "subscribe", "api_version": 2, "streams": []string{"transactions", "ledger"}}); err != nil {
		return fmt.Errorf("subscribe AMM discovery: %w", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var ack ammMessage
	if err := conn.ReadJSON(&ack); err != nil {
		return fmt.Errorf("read AMM subscription acknowledgement: %w", err)
	}
	if ack.ID != id || ack.Type != "response" || ack.Status != "success" || ack.Error != "" {
		return fmt.Errorf("AMM subscription rejected: %s %s", ack.Status, ack.Error)
	}
	emit(AMMStreamUpdate{})
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	for {
		var message ammMessage
		if err := conn.ReadJSON(&message); err != nil {
			return fmt.Errorf("read AMM stream: %w", err)
		}
		if message.Error != "" {
			return fmt.Errorf("AMM stream error: %s", message.Error)
		}
		if message.Type == "ledgerClosed" && message.LedgerIndex != 0 {
			_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			emit(AMMStreamUpdate{LedgerIndex: message.LedgerIndex})
			continue
		}
		creation, err := creationFromMessage(message)
		// A malformed successful AMMCreate is a coverage problem, not silent success.
		if err != nil {
			return fmt.Errorf("decode AMM creation: %w", err)
		}
		if creation != nil {
			emit(AMMStreamUpdate{Creation: creation})
		}
	}
}
