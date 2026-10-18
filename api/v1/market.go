package v1

import (
	"github.com/Kirk-V/XRPLTrading/internal/domain"
	"time"
)

type BookLevel struct {
	Price    string `json:"price"`
	Size     string `json:"size"`
	Owner    string `json:"owner"`
	Sequence uint32 `json:"sequence"`
}

type OrderBook struct {
	Pair        domain.Pair `json:"pair"`
	Quote       Quote       `json:"quote"`
	Bids        []BookLevel `json:"bids"`
	Asks        []BookLevel `json:"asks"`
	LedgerIndex uint32      `json:"ledger_index"`
	ObservedAt  time.Time   `json:"observed_at"`
}

type MarketState struct {
	Network        string        `json:"network"`
	ReadOnly       bool          `json:"read_only"`
	Endpoint       string        `json:"endpoint"`
	Connection     string        `json:"connection"`
	Error          string        `json:"error,omitempty"`
	LedgerIndex    uint32        `json:"ledger_index"`
	LedgerClosedAt time.Time     `json:"ledger_closed_at"`
	ObservedAt     time.Time     `json:"observed_at"`
	Markets        []domain.Pair `json:"markets"`
	AMMDiscovery   AMMDiscovery  `json:"amm_discovery"`
	AMMCreations   []AMMCreation `json:"amm_creations"`
}

// AMMCreation is a creation event, not a current pool balance or execution quote.
type AMMCreation struct {
	Pair            domain.Pair `json:"pair"`
	LedgerIndex     uint32      `json:"ledger_index"`
	TransactionHash string      `json:"transaction_hash"`
	ObservedAt      time.Time   `json:"observed_at"`
}

type AMMDiscovery struct {
	Connection    string    `json:"connection"`
	Error         string    `json:"error,omitempty"`
	LedgerIndex   uint32    `json:"ledger_index"`
	LastMessageAt time.Time `json:"last_message_at"`
	Interrupted   bool      `json:"interrupted"`
}
