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

func TestAMMStatusUsesValidatedFundedPool(t *testing.T) {
	cases := []struct {
		name, response, want string
		failed               bool
	}{
		{"funded", fmt.Sprintf(`{"result":{"status":"success","validated":true,"ledger_index":42,"amm":{"account":"pool","amount":"1000000","amount2":{"currency":"USD","issuer":%q,"value":"2"}}}}`, testIssuer), "active", false},
		{"reversed", fmt.Sprintf(`{"result":{"status":"success","validated":true,"ledger_index":42,"amm":{"account":"pool","amount":{"currency":"USD","issuer":%q,"value":"2"},"amount2":"1000000"}}}`, testIssuer), "active", false},
		{"missing", `{"result":{"status":"error","error":"actNotFound","error_message":"Account not found."}}`, "none", false},
		{"frozen", fmt.Sprintf(`{"result":{"status":"success","validated":true,"ledger_index":42,"amm":{"account":"pool","amount":"1000000","amount2":{"currency":"USD","issuer":%q,"value":"2"},"asset2_frozen":true}}}`, testIssuer), "inactive", false},
		{"zero reserve", fmt.Sprintf(`{"result":{"status":"success","validated":true,"ledger_index":42,"amm":{"account":"pool","amount":"0","amount2":{"currency":"USD","issuer":%q,"value":"2"}}}}`, testIssuer), "inactive", false},
		{"other upstream error", `{"result":{"status":"error","error":"tooBusy"}}`, "unknown", true},
		{"unvalidated", fmt.Sprintf(`{"result":{"status":"success","validated":false,"ledger_index":42,"amm":{"account":"pool","amount":"1000000","amount2":{"currency":"USD","issuer":%q,"value":"2"}}}}`, testIssuer), "unknown", true},
		{"wrong token", `{"result":{"status":"success","validated":true,"ledger_index":42,"amm":{"account":"pool","amount":"1000000","amount2":{"currency":"EUR","issuer":"rOther","value":"2"}}}}`, "unknown", true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Method string           `json:"method"`
					Params []map[string]any `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.Method != "amm_info" || len(body.Params) != 1 || body.Params[0]["ledger_index"] != "validated" {
					t.Error("unexpected lookup", body)
				}
				fmt.Fprint(w, test.response)
			}))
			defer upstream.Close()
			cfg := config.Default()
			cfg.JSONRPC = upstream.URL
			client, _ := NewAPIClient(cfg)
			pair, _ := domain.NewXRPTokenPair(domain.Issued("USD", testIssuer))
			status, err := client.AMMStatus(context.Background(), pair)
			if (err != nil) != test.failed || status.State != test.want {
				t.Fatalf("status=%+v err=%v", status, err)
			}
			if status.State == "active" && (status.LedgerIndex != 42 || status.CheckedAt.IsZero()) {
				t.Fatal("missing validated cursor", status)
			}
		})
	}
}
