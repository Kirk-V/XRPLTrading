package xrpl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Kirk-V/XRPLTrading/internal/config"
	"github.com/gorilla/websocket"
)

func ammPayload() map[string]any {
	return map[string]any{
		"type": "transaction", "validated": true, "ledger_index": 42, "hash": strings.Repeat("A", 64),
		"tx_json": map[string]any{"TransactionType": "AMMCreate", "Amount": "1000000", "Amount2": map[string]any{"currency": "usd", "issuer": testIssuer, "value": "10"}},
		"meta":    map[string]any{"TransactionResult": "tesSUCCESS"},
	}
}

func TestAMMCreationFiltering(t *testing.T) {
	cases := []struct {
		name    string
		change  func(map[string]any)
		want    bool
		wantErr bool
	}{
		{"validated creation", func(m map[string]any) {}, true, false},
		{"reversed amounts", func(m map[string]any) {
			tx := m["tx_json"].(map[string]any)
			tx["Amount"], tx["Amount2"] = tx["Amount2"], tx["Amount"]
		}, true, false},
		{"legacy transaction", func(m map[string]any) {
			tx := m["tx_json"].(map[string]any)
			tx["hash"] = m["hash"]
			m["transaction"] = tx
			delete(m, "tx_json")
			delete(m, "hash")
		}, true, false},
		{"unvalidated", func(m map[string]any) { m["validated"] = false }, false, false},
		{"failed", func(m map[string]any) { m["meta"] = map[string]any{"TransactionResult": "tecAMM_FAILED"} }, false, false},
		{"missing metadata", func(m map[string]any) { delete(m, "meta") }, false, false},
		{"other transaction", func(m map[string]any) { m["tx_json"].(map[string]any)["TransactionType"] = "AMMDeposit" }, false, false},
		{"token token", func(m map[string]any) {
			m["tx_json"].(map[string]any)["Amount"] = map[string]any{"currency": "EUR", "issuer": testIssuer, "value": "2"}
		}, false, false},
		{"malformed amount", func(m map[string]any) { m["tx_json"].(map[string]any)["Amount"] = "invalid" }, false, true},
		{"zero amount", func(m map[string]any) { m["tx_json"].(map[string]any)["Amount"] = "0" }, false, true},
		{"missing ledger", func(m map[string]any) { delete(m, "ledger_index") }, false, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			raw := ammPayload()
			test.change(raw)
			data, _ := json.Marshal(raw)
			var message ammMessage
			if err := json.Unmarshal(data, &message); err != nil {
				t.Fatal(err)
			}
			creation, err := creationFromMessage(message)
			if (err != nil) != test.wantErr || (creation != nil) != test.want {
				t.Fatalf("creation=%+v err=%v", creation, err)
			}
			if creation != nil && (!creation.Pair.Base.IsXRP() || creation.Pair.Quote.Currency != "usd" || creation.TransactionHash != strings.Repeat("A", 64)) {
				t.Fatalf("identity lost: %+v", creation)
			}
		})
	}
}

func TestWatchAMMsSubscriptionAndCancellation(t *testing.T) {
	serverDone := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		var request struct {
			ID      uint64   `json:"id"`
			Command string   `json:"command"`
			Version int      `json:"api_version"`
			Streams []string `json:"streams"`
		}
		if err := conn.ReadJSON(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Command != "subscribe" || request.Version != 2 || strings.Join(request.Streams, ",") != "transactions,ledger" {
			t.Errorf("bad subscription: %+v", request)
		}
		_ = conn.WriteJSON(map[string]any{"id": request.ID, "type": "response", "status": "success"})
		_ = conn.WriteJSON(map[string]any{"type": "ledgerClosed", "ledger_index": 42})
		unvalidated := ammPayload()
		unvalidated["validated"] = false
		_ = conn.WriteJSON(unvalidated)
		_ = conn.WriteJSON(ammPayload())
		_, _, _ = conn.ReadMessage() // Client cancellation must close the socket.
	}))
	defer upstream.Close()
	cfg := config.Default()
	cfg.WebSocket = "ws" + strings.TrimPrefix(upstream.URL, "http")
	client, _ := NewAPIClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	calls := 0
	creations := 0
	err := client.WatchAMMs(ctx, func(update AMMStreamUpdate) {
		calls++
		if update.Creation != nil {
			creations++
			cancel()
		}
	})
	if err == nil || calls != 3 || creations != 1 {
		t.Fatalf("calls=%d creations=%d err=%v", calls, creations, err)
	}
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("socket not closed after cancellation")
	}
}

func TestWatchAMMsRejectsSubscription(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var request map[string]any
		_ = conn.ReadJSON(&request)
		_ = conn.WriteJSON(map[string]any{"id": request["id"], "type": "response", "status": "error", "error": "reportingUnsupported"})
	}))
	defer upstream.Close()
	cfg := config.Default()
	cfg.WebSocket = "ws" + strings.TrimPrefix(upstream.URL, "http")
	client, _ := NewAPIClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := client.WatchAMMs(ctx, func(AMMStreamUpdate) { t.Error("rejected subscription announced connected") })
	if err == nil || !strings.Contains(fmt.Sprint(err), "reportingUnsupported") {
		t.Fatal("missing rejection", err)
	}
}
