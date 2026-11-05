package xrpl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kirk-V/XRPLTrading/internal/config"
	"github.com/Kirk-V/XRPLTrading/internal/domain"
)

const testIssuer = "rQGbw4Q5jqq2t25r4vjsyRksXvoM1o2Jf6"

func TestMarketBookUsesRPCAndPinsBothSidesToValidatedLedger(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request struct {
			Method  string           `json:"method"`
			Command string           `json:"command"`
			Params  []map[string]any `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Method != "book_offers" || request.Command != "" || len(request.Params) != 1 {
			t.Errorf("bad request: %+v", request)
			http.Error(w, "bad request", 400)
			return
		}
		args := request.Params[0]
		if args["api_version"] != float64(2) {
			t.Error("API version must be inside params")
		}
		if calls == 1 {
			if args["ledger_index"] != "validated" {
				t.Error("first side not validated")
			}
			if args["taker_gets"].(map[string]any)["currency"] != "XRP" {
				t.Error("ask direction reversed")
			}
			fmt.Fprintf(w, `{"result":{"status":"success","validated":true,"ledger_index":42,"offers":[{"Account":"maker","Sequence":1,"TakerGets":"10000000","TakerPays":{"currency":"USD","issuer":%q,"value":"6"},"taker_gets_funded":"5000000","taker_pays_funded":{"currency":"USD","issuer":%q,"value":"3"}}]}}`, testIssuer, testIssuer)
		} else {
			if args["ledger_index"] != float64(42) {
				t.Error("second side not pinned")
			}
			if args["taker_gets"].(map[string]any)["currency"] != "USD" {
				t.Error("bid direction reversed")
			}
			fmt.Fprintf(w, `{"result":{"status":"success","validated":true,"ledger_index":42,"offers":[{"Account":"maker","Sequence":2,"TakerGets":{"currency":"USD","issuer":%q,"value":"4"},"TakerPays":"10000000"}]}}`, testIssuer)
		}
	}))
	defer upstream.Close()
	cfg := config.Default()
	cfg.JSONRPC = upstream.URL
	client, _ := NewAPIClient(cfg)
	pair, _ := domain.NewXRPTokenPair(domain.Issued("USD", testIssuer))
	book, err := client.MarketBook(context.Background(), pair)
	if err != nil {
		t.Fatal(err)
	}
	if book.Quote.Ask != "0.6" || book.Quote.Bid != "0.4" || book.Quote.Mid != "0.5" || book.Quote.Spread != "0.2" {
		t.Fatalf("wrong quote: %+v", book.Quote)
	}
	if book.Asks[0].Size != "5" || book.Bids[0].Size != "10" {
		t.Fatalf("wrong funded sizes: %+v", book)
	}
}

func TestRPCRejectsNestedErrorsAndUnvalidatedBooks(t *testing.T) {
	for _, response := range []string{`{"result":{"status":"error","error":"lgrNotFound"}}`, `{"result":{"ledger_index":42,"validated":false,"offers":[]}}`, `{"result":null}`} {
		t.Run(response, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, response) }))
			defer upstream.Close()
			cfg := config.Default()
			cfg.JSONRPC = upstream.URL
			client, _ := NewAPIClient(cfg)
			pair, _ := domain.NewXRPTokenPair(domain.Issued("USD", testIssuer))
			if _, err := client.MarketBook(context.Background(), pair); err == nil {
				t.Fatal("expected upstream response rejection")
			}
		})
	}
}

func TestBookChangesAndEmptyBookWithoutOptionalValidatedFlag(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		if request.Command == "book_changes" {
			fmt.Fprintf(w, `{"result":{"validated":true,"ledger_index":42,"ledger_time":100,"changes":[{"currency_a":"XRP_drops","currency_b":%q}]}}`, testIssuer+"/USD")
		} else {
			fmt.Fprint(w, `{"result":{"ledger_index":42,"offers":[]}}`)
		}
	}))
	defer upstream.Close()
	cfg := config.Default()
	cfg.JSONRPC = upstream.URL
	client, _ := NewAPIClient(cfg)
	changes, err := client.BookChanges(context.Background())
	if err != nil || changes.LedgerIndex != 42 || len(changes.Changes) != 1 {
		t.Fatalf("changes=%+v err=%v", changes, err)
	}
	pair, _ := domain.NewXRPTokenPair(domain.Issued("USD", testIssuer))
	book, err := client.MarketBook(context.Background(), pair)
	if err != nil {
		t.Fatal(err)
	}
	if len(book.Bids) != 0 || len(book.Asks) != 0 || book.Quote.Mid != "" {
		t.Fatalf("empty book invented a quote: %+v", book)
	}
}

func TestAmountsAndMarketIdentity(t *testing.T) {
	asset := domain.Asset{Currency: "usd", Issuer: testIssuer}
	n, err := amount(json.RawMessage(fmt.Sprintf(`{"currency":"usd","issuer":%q,"value":"1e-20"}`, testIssuer)), asset)
	if err != nil || decimal(n) != "1e-20" {
		t.Fatalf("tiny amount: %v %v", n, err)
	}
	for _, raw := range []string{`"1/2"`, `"-1"`, `"1e999999999"`, `"0.1"`} {
		if _, err := amount(json.RawMessage(raw), domain.XRP()); err == nil {
			t.Errorf("accepted invalid XRP amount %s", raw)
		}
	}
	pair, err := ValidateMarketInput("usd", testIssuer)
	if err != nil || pair.Quote.Currency != "usd" {
		t.Fatal("currency case was lost", pair, err)
	}
	if _, err := ValidateMarketInput("USD", "rFake"); err == nil {
		t.Fatal("accepted invalid address")
	}
}
