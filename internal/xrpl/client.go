package xrpl

import (
	"context"

	"github.com/yourname/xrpltrading/internal/domain"
)

// ReadClient is the narrow, read-only XRPL boundary used by the application layer.
// A live WebSocket/JSON-RPC implementation and a replay fake can both satisfy it.
type ReadClient interface {
	Connect(ctx context.Context) error
	BookOffers(ctx context.Context, pair domain.Pair, limit int) ([]Offer, error)
	SubscribeBook(ctx context.Context, pair domain.Pair) (<-chan BookUpdate, error)
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
