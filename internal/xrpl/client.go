package xrpl

import (
	"context"

	"github.com/Kirk-V/XRPLTrading/internal/domain"
)

// ReadClient is the narrow, read-only XRPL boundary used by the application layer.
// A live WebSocket/JSON-RPC implementation and a replay fake can both satisfy it.
type ReadClient interface {
	Connect(ctx context.Context) error
	BookOffers(ctx context.Context, pair domain.Pair, limit int) ([]Offer, error)
	SubscribeBook(ctx context.Context, pair domain.Pair) (<-chan BookUpdate, error)
	SubscribeLedger(ctx context.Context) (<-chan LedgerUpdate, error)
}

type Offer struct {
	Sequence uint32 `json:"sequence"`
	Owner    string `json:"owner"`
	Pays     string `json:"pays"`
	Gets     string `json:"gets"`
}

type BookUpdate struct {
	LedgerIndex uint32  `json:"ledger_index"`
	Offers      []Offer `json:"offers"`
}

// LedgerUpdate represents a validated ledgerClosed notification.
type LedgerUpdate struct {
	LedgerIndex uint32 `json:"ledger_index"`
	LedgerHash  string `json:"ledger_hash"`
	LedgerTime  uint32 `json:"ledger_time"`
	TxnCount    uint32 `json:"txn_count"`
}
