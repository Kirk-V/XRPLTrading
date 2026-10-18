package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	v1 "github.com/Kirk-V/XRPLTrading/api/v1"
	"github.com/Kirk-V/XRPLTrading/internal/config"
	"github.com/Kirk-V/XRPLTrading/internal/domain"
	"github.com/Kirk-V/XRPLTrading/internal/xrpl"
)

func TestAMMDiscoveryDedupAndCapacity(t *testing.T) {
	service := New(&fakeClient{}, config.Default())
	service.refresh(context.Background())
	pair := service.snapshot().Markets[0]
	creation := v1.AMMCreation{Pair: pair, LedgerIndex: 42, TransactionHash: "hash", ObservedAt: time.Now().UTC()}
	service.onAMMUpdate(xrpl.AMMStreamUpdate{})
	service.onAMMUpdate(xrpl.AMMStreamUpdate{Creation: &creation})
	service.onAMMUpdate(xrpl.AMMStreamUpdate{Creation: &creation})
	state := service.snapshot()
	if len(state.Markets) != 1 || len(state.AMMCreations) != 1 {
		t.Fatal("duplicate or existing pair lost creation", state)
	}
	// Fill the catalog, then verify new AMMs still become visible.
	for i := 0; i < 200; i++ {
		newPair, _ := domain.NewXRPTokenPair(domain.Asset{Currency: fmt.Sprintf("%03d", i), Issuer: pair.Quote.Issuer})
		item := v1.AMMCreation{Pair: newPair, TransactionHash: fmt.Sprint(i)}
		service.onAMMUpdate(xrpl.AMMStreamUpdate{Creation: &item})
	}
	state = service.snapshot()
	if len(state.Markets) != 200 || len(state.AMMCreations) != 100 || state.AMMCreations[0].Pair.Quote.Currency != "199" {
		t.Fatal("unbounded or missing new pool")
	}
	state.AMMCreations[0].TransactionHash = "changed"
	if service.snapshot().AMMCreations[0].TransactionHash == "changed" {
		t.Fatal("snapshot aliases service state")
	}
	service.onAMMUpdate(xrpl.AMMStreamUpdate{LedgerIndex: 42})
	service.onAMMUpdate(xrpl.AMMStreamUpdate{LedgerIndex: 44})
	if !service.snapshot().AMMDiscovery.Interrupted {
		t.Fatal("ledger gap hidden")
	}
	service.state.AMMDiscovery.LastMessageAt = time.Now().Add(-time.Minute)
	if service.snapshot().AMMDiscovery.Connection != "stale" {
		t.Fatal("silent stream marked current")
	}
}

type reconnectClient struct {
	fakeClient
	calls   int
	cancel  context.CancelFunc
	service *Service
}

func (c *reconnectClient) WatchAMMs(ctx context.Context, emit func(xrpl.AMMStreamUpdate)) error {
	c.calls++
	if c.calls == 1 {
		return errors.New("disconnected")
	}
	if c.service.snapshot().AMMDiscovery.Connection != "reconnecting" {
		return errors.New("missing reconnect state")
	}
	emit(xrpl.AMMStreamUpdate{LedgerIndex: 50})
	c.cancel()
	return ctx.Err()
}
func TestAMMReconnectAndCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := &reconnectClient{cancel: cancel}
	service := New(client, config.Default())
	client.service = service
	service.runAMMs(ctx)
	state := service.snapshot().AMMDiscovery
	if client.calls != 2 || state.Connection != "connected" || !state.Interrupted || state.Error != "" {
		t.Fatalf("bad recovery: calls=%d state=%+v", client.calls, state)
	}
}
