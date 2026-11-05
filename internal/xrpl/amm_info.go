package xrpl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	v1 "github.com/Kirk-V/XRPLTrading/api/v1"
	"github.com/Kirk-V/XRPLTrading/internal/domain"
)

// AMMStatus uses the latest validated ledger to distinguish a funded pool
// from a missing AMM. Other upstream failures are returned to the caller.
func (c *APIClient) AMMStatus(ctx context.Context, pair domain.Pair) (v1.AMMStatus, error) {
	status := v1.AMMStatus{Pair: pair, State: "unknown"}
	if err := pair.Validate(); err != nil {
		return status, err
	}
	if !pair.Base.IsXRP() {
		return status, fmt.Errorf("AMM lookup base must be XRP")
	}
	var result struct {
		LedgerIndex uint32 `json:"ledger_index"`
		Validated   *bool  `json:"validated"`
		AMM         struct {
			Account      string          `json:"account"`
			Amount       json.RawMessage `json:"amount"`
			Amount2      json.RawMessage `json:"amount2"`
			AssetFrozen  bool            `json:"asset_frozen"`
			Asset2Frozen bool            `json:"asset2_frozen"`
		} `json:"amm"`
	}
	err := c.call(ctx, c.config.JSONRPC, "amm_info", map[string]any{
		"asset": currencyObject(pair.Base), "asset2": currencyObject(pair.Quote), "ledger_index": "validated",
	}, &result)
	if err != nil {
		if strings.Contains(err.Error(), "actNotFound") {
			status.State = "none"
			status.CheckedAt = time.Now().UTC()
			return status, nil
		}
		return status, err
	}
	if result.LedgerIndex == 0 || (result.Validated != nil && !*result.Validated) {
		return status, errors.New("endpoint did not return a validated AMM")
	}
	if result.AMM.Account == "" {
		return status, errors.New("AMM response missing pool account")
	}
	if result.AMM.AssetFrozen || result.AMM.Asset2Frozen {
		status.State = "inactive"
		status.LedgerIndex = result.LedgerIndex
		status.CheckedAt = time.Now().UTC()
		return status, nil
	}
	assets := []domain.Asset{pair.Base, pair.Quote}
	for i, raw := range []json.RawMessage{result.AMM.Amount, result.AMM.Amount2} {
		var drops string
		if json.Unmarshal(raw, &drops) == nil {
			if i == 1 {
				assets[0], assets[1] = assets[1], assets[0]
			}
			break
		}
	}
	for i, raw := range []json.RawMessage{result.AMM.Amount, result.AMM.Amount2} {
		n, err := amount(raw, assets[i])
		if err != nil {
			return status, fmt.Errorf("decode AMM reserve: %w", err)
		}
		if n.Sign() <= 0 {
			status.State = "inactive"
			status.LedgerIndex = result.LedgerIndex
			status.CheckedAt = time.Now().UTC()
			return status, nil
		}
	}
	status.State = "active"
	status.LedgerIndex = result.LedgerIndex
	status.CheckedAt = time.Now().UTC()
	return status, nil
}
