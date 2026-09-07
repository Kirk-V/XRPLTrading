package market

import (
	"context"
	"testing"
	"time"

	"github.com/Kirk-V/XRPLTrading/internal/domain"
	"github.com/Kirk-V/XRPLTrading/internal/xrpl"
)

type fakeReadClient struct {
	connects int
	books    int
	streams  []<-chan xrpl.BookUpdate
	ledgers  []<-chan xrpl.LedgerUpdate
}

func (f *fakeReadClient) Connect(context.Context) error {
	f.connects++
	return nil
}

func (f *fakeReadClient) BookOffers(context.Context, domain.Pair, int) ([]xrpl.Offer, error) {
	f.books++
	return []xrpl.Offer{{Sequence: uint32(f.books)}}, nil
}

func (f *fakeReadClient) SubscribeBook(context.Context, domain.Pair) (<-chan xrpl.BookUpdate, error) {
	stream := f.streams[0]
	f.streams = f.streams[1:]
	return stream, nil
}

func (f *fakeReadClient) SubscribeLedger(context.Context) (<-chan xrpl.LedgerUpdate, error) {
	stream := f.ledgers[0]
	f.ledgers = f.ledgers[1:]
	return stream, nil
}

func TestIngestorReloadsSnapshotAfterStreamDisconnect(t *testing.T) {
	pair, err := domain.NewXRPTokenPair(domain.Issued("USD", "rIssuer"))
	if err != nil {
		t.Fatal(err)
	}
	first := make(chan xrpl.BookUpdate, 1)
	first <- xrpl.BookUpdate{LedgerIndex: 10, Offers: []xrpl.Offer{{Sequence: 7}}}
	close(first)
	second := make(chan xrpl.BookUpdate)
	fake := &fakeReadClient{streams: []<-chan xrpl.BookUpdate{first, second}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ingestor := Ingestor{
		Client:         fake,
		InitialBackoff: time.Nanosecond,
		MaxBackoff:     time.Nanosecond,
		Sleep:          func(context.Context, time.Duration) error { return nil },
	}
	updates, err := ingestor.Run(ctx, pair)
	if err != nil {
		t.Fatal(err)
	}

	firstEvent := <-updates
	if !firstEvent.Snapshot || len(firstEvent.Offers) != 1 {
		t.Fatalf("unexpected initial snapshot: %+v", firstEvent)
	}
	streamEvent := <-updates
	if streamEvent.Snapshot || streamEvent.LedgerIndex != 10 {
		t.Fatalf("unexpected stream event: %+v", streamEvent)
	}
	recovery := <-updates
	if !recovery.Snapshot || len(recovery.Offers) != 1 {
		t.Fatalf("unexpected recovery snapshot: %+v", recovery)
	}
	if fake.connects != 2 || fake.books != 2 {
		t.Fatalf("expected reconnect and reload, connects=%d books=%d", fake.connects, fake.books)
	}
	cancel()
}

func TestLedgerIngestorReconnectsAfterStreamDisconnect(t *testing.T) {
	first := make(chan xrpl.LedgerUpdate, 1)
	first <- xrpl.LedgerUpdate{LedgerIndex: 100, LedgerHash: "first"}
	close(first)
	second := make(chan xrpl.LedgerUpdate)
	fake := &fakeReadClient{ledgers: []<-chan xrpl.LedgerUpdate{first, second}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updates, err := (LedgerIngestor{
		Client:         fake,
		InitialBackoff: time.Nanosecond,
		MaxBackoff:     time.Nanosecond,
		Sleep:          func(context.Context, time.Duration) error { return nil },
	}).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if event := <-updates; event.Update.LedgerIndex != 100 {
		t.Fatalf("unexpected ledger event: %+v", event)
	}
	// The second subscription is now active. Cancellation proves the
	// reconnect loop did not spin or leak after the first stream closed.
	cancel()
	select {
	case _, ok := <-updates:
		if ok {
			for range updates {
			}
		}
	case <-time.After(time.Second):
		t.Fatal("ledger ingestor did not stop after cancellation")
	}
	if fake.connects != 2 {
		t.Fatalf("expected two connections, got %d", fake.connects)
	}
}

func TestIngestorStopsOnContextCancellation(t *testing.T) {
	pair, _ := domain.NewXRPTokenPair(domain.Issued("USD", "rIssuer"))
	fake := &fakeReadClient{streams: []<-chan xrpl.BookUpdate{make(chan xrpl.BookUpdate)}}
	ctx, cancel := context.WithCancel(context.Background())
	updates, err := (Ingestor{Client: fake}).Run(ctx, pair)
	if err != nil {
		t.Fatal(err)
	}
	<-updates
	cancel()
	select {
	case _, ok := <-updates:
		if ok {
			for range updates {
			}
		}
	case <-time.After(time.Second):
		t.Fatal("ingestor did not stop after cancellation")
	}
}
