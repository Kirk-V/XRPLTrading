// Package v1 contains versioned local API read models. Financial quantities
// are strings so clients never round XRPL decimal values through float64.
package v1

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Kirk-V/XRPLTrading/internal/domain"
)

const Version = "v1"

type Quote struct {
	Pair        domain.Pair `json:"pair"`
	Bid         string      `json:"bid,omitempty"`
	Ask         string      `json:"ask,omitempty"`
	Mid         string      `json:"mid,omitempty"`
	Spread      string      `json:"spread,omitempty"`
	LedgerIndex uint32      `json:"ledger_index"`
	ObservedAt  time.Time   `json:"observed_at"`
}

type Candle struct {
	Pair        domain.Pair `json:"pair"`
	Start       time.Time   `json:"start"`
	End         time.Time   `json:"end"`
	Open        string      `json:"open"`
	High        string      `json:"high"`
	Low         string      `json:"low"`
	Close       string      `json:"close"`
	Volume      string      `json:"volume"`
	TradeCount  uint64      `json:"trade_count"`
	LedgerIndex uint32      `json:"ledger_index"`
}

type Wallet struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`
	Label     string    `json:"label,omitempty"`
	WatchOnly bool      `json:"watch_only"`
	CreatedAt time.Time `json:"created_at"`
}

type Balance struct {
	WalletID    string       `json:"wallet_id"`
	Asset       domain.Asset `json:"asset"`
	Total       string       `json:"total"`
	Spendable   string       `json:"spendable"`
	ReserveHeld string       `json:"reserve_held,omitempty"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type OrderStatus string

const (
	OrderSubmitted       OrderStatus = "submitted"
	OrderValidated       OrderStatus = "validated"
	OrderPartiallyFilled OrderStatus = "partially_filled"
	OrderFilled          OrderStatus = "filled"
	OrderCanceled        OrderStatus = "canceled"
	OrderExpired         OrderStatus = "expired"
	OrderFailed          OrderStatus = "failed"
)

type Order struct {
	ID            string      `json:"id"`
	WalletID      string      `json:"wallet_id"`
	Pair          domain.Pair `json:"pair"`
	Side          string      `json:"side"`
	Amount        string      `json:"amount"`
	LimitPrice    string      `json:"limit_price,omitempty"`
	FilledAmount  string      `json:"filled_amount"`
	Status        OrderStatus `json:"status"`
	TransactionID string      `json:"transaction_id,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type TransactionStatus string

const (
	TransactionPending   TransactionStatus = "pending"
	TransactionValidated TransactionStatus = "validated"
	TransactionFailed    TransactionStatus = "failed"
)

type Transaction struct {
	ID           string            `json:"id"`
	WalletID     string            `json:"wallet_id"`
	Hash         string            `json:"hash,omitempty"`
	Status       TransactionStatus `json:"status"`
	EngineResult string            `json:"engine_result,omitempty"`
	Fee          string            `json:"fee,omitempty"`
	LedgerIndex  uint32            `json:"ledger_index,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

func (w Wallet) Validate() error {
	if strings.TrimSpace(w.ID) == "" || strings.TrimSpace(w.Address) == "" {
		return errors.New("wallet id and address are required")
	}
	return nil
}

func (b Balance) Validate() error {
	if strings.TrimSpace(b.WalletID) == "" {
		return errors.New("balance wallet id is required")
	}
	if err := b.Asset.Validate(); err != nil {
		return fmt.Errorf("balance asset: %w", err)
	}
	if strings.TrimSpace(b.Total) == "" || strings.TrimSpace(b.Spendable) == "" {
		return errors.New("balance total and spendable amounts are required")
	}
	return nil
}

func (o Order) Validate() error {
	if strings.TrimSpace(o.ID) == "" || strings.TrimSpace(o.WalletID) == "" {
		return errors.New("order id and wallet id are required")
	}
	if err := o.Pair.Validate(); err != nil {
		return fmt.Errorf("order pair: %w", err)
	}
	if o.Side != "buy" && o.Side != "sell" {
		return fmt.Errorf("invalid order side %q", o.Side)
	}
	if strings.TrimSpace(o.Amount) == "" || o.Status == "" {
		return errors.New("order amount and status are required")
	}
	return nil
}

func (t Transaction) Validate() error {
	if strings.TrimSpace(t.ID) == "" || strings.TrimSpace(t.WalletID) == "" {
		return errors.New("transaction id and wallet id are required")
	}
	if t.Status == "" {
		return errors.New("transaction status is required")
	}
	return nil
}
