package market

import (
	"context"
	"errors"
	"time"

	"github.com/Kirk-V/XRPLTrading/internal/domain"
	"github.com/Kirk-V/XRPLTrading/internal/storage"
	"github.com/Kirk-V/XRPLTrading/internal/xrpl"
)

const (
	bookSnapshotEvent = "BookSnapshot"
	bookUpdateEvent   = "BookUpdated"
)

// BookEvent is the normalized stream consumed by market read models and the UI.
// Snapshot is true for the initial load and for every recovery load after a
// disconnect. A recovery snapshot is the service's backfill boundary: it
// prevents a reconnect from silently presenting an incomplete book.
type BookEvent struct {
	Pair        domain.Pair
	LedgerIndex uint32
	Offers      []xrpl.Offer
	Snapshot    bool
}

// Ingestor owns the lifecycle of one book stream. It reconnects with bounded
// backoff and reloads the complete book before publishing the stream as current.
type Ingestor struct {
	Client         xrpl.ReadClient
	Limit          int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Sleep          func(context.Context, time.Duration) error
	Journal        storage.Journal
}

func (i Ingestor) Run(ctx context.Context, pair domain.Pair) (<-chan BookEvent, error) {
	if i.Client == nil {
		return nil, errors.New("market ingestor requires an XRPL client")
	}
	if err := pair.Validate(); err != nil {
		return nil, err
	}
	if i.Limit <= 0 {
		i.Limit = 100
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

	output := make(chan BookEvent)
	go i.run(ctx, pair, output)
	return output, nil
}

func (i Ingestor) run(ctx context.Context, pair domain.Pair, output chan<- BookEvent) {
	defer close(output)
	backoff := i.InitialBackoff

	for ctx.Err() == nil {
		if err := i.Client.Connect(ctx); err == nil {
			if offers, err := i.Client.BookOffers(ctx, pair, i.Limit); err == nil {
				event := BookEvent{Pair: pair, Offers: offers, Snapshot: true}
				if !i.persist(ctx, bookSnapshotEvent, event) || !sendBookEvent(ctx, output, event) {
					return
				}
				if stream, err := i.Client.SubscribeBook(ctx, pair); err == nil {
					backoff = i.InitialBackoff
					if consumeBookStream(ctx, pair, stream, output, i.Journal) {
						return
					}
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

func consumeBookStream(ctx context.Context, pair domain.Pair, stream <-chan xrpl.BookUpdate, output chan<- BookEvent, journal storage.Journal) bool {
	for {
		select {
		case <-ctx.Done():
			return true
		case update, ok := <-stream:
			if !ok {
				return false
			}
			event := BookEvent{Pair: pair, LedgerIndex: update.LedgerIndex, Offers: update.Offers}
			if !persist(ctx, journal, bookUpdateEvent, update.LedgerIndex, event) || !sendBookEvent(ctx, output, event) {
				return true
			}
		}
	}
}

func (i Ingestor) persist(ctx context.Context, eventType string, event BookEvent) bool {
	return persist(ctx, i.Journal, eventType, event.LedgerIndex, event)
}

func persist(ctx context.Context, journal storage.Journal, eventType string, ledgerIndex uint32, payload any) bool {
	if journal == nil {
		return true
	}
	event, err := storage.NewEvent(eventType, ledgerIndex, payload)
	if err != nil {
		return false
	}
	_, err = journal.Append(ctx, event)
	return err == nil
}

func sendBookEvent(ctx context.Context, output chan<- BookEvent, event BookEvent) bool {
	select {
	case output <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

func sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
