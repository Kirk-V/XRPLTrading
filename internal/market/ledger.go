package market

import (
	"context"
	"errors"
	"time"

	"github.com/Kirk-V/XRPLTrading/internal/storage"
	"github.com/Kirk-V/XRPLTrading/internal/xrpl"
)

const ledgerClosedEvent = "LedgerClosed"

// LedgerEvent is the validated-ledger cursor shared by market ingestion and
// future durable backfill/replay components.
type LedgerEvent struct {
	Update xrpl.LedgerUpdate
}

// LedgerIngestor maintains a reconnecting validated-ledger stream.
type LedgerIngestor struct {
	Client         xrpl.ReadClient
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Sleep          func(context.Context, time.Duration) error
	Journal        storage.Journal
}

func (i LedgerIngestor) Run(ctx context.Context) (<-chan LedgerEvent, error) {
	if i.Client == nil {
		return nil, errors.New("ledger ingestor requires an XRPL client")
	}
	if i.InitialBackoff <= 0 {
		i.InitialBackoff = 250 * time.Millisecond
	}
	if i.MaxBackoff <= 0 {
		i.MaxBackoff = 5 * time.Second
	}
	if i.Sleep == nil {
		i.Sleep = sleep
	}

	output := make(chan LedgerEvent)
	go i.run(ctx, output)
	return output, nil
}

func (i LedgerIngestor) run(ctx context.Context, output chan<- LedgerEvent) {
	defer close(output)
	backoff := i.InitialBackoff
	for ctx.Err() == nil {
		if err := i.Client.Connect(ctx); err == nil {
			if stream, err := i.Client.SubscribeLedger(ctx); err == nil {
				backoff = i.InitialBackoff
				if consumeLedgerStream(ctx, stream, output, i.Journal) {
					return
				}
			}
		}
		if err := i.Sleep(ctx, backoff); err != nil {
			return
		}
		if backoff < i.MaxBackoff {
			backoff *= 2
			if backoff > i.MaxBackoff {
				backoff = i.MaxBackoff
			}
		}
	}
}

func consumeLedgerStream(ctx context.Context, stream <-chan xrpl.LedgerUpdate, output chan<- LedgerEvent, journal storage.Journal) bool {
	for {
		select {
		case <-ctx.Done():
			return true
		case update, ok := <-stream:
			if !ok {
				return false
			}
			if !persist(ctx, journal, ledgerClosedEvent, update.LedgerIndex, update) {
				return true
			}
			select {
			case output <- LedgerEvent{Update: update}:
			case <-ctx.Done():
				return true
			}
		}
	}
}
