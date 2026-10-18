package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "github.com/Kirk-V/XRPLTrading/api/v1"
	"github.com/Kirk-V/XRPLTrading/internal/config"
	"github.com/Kirk-V/XRPLTrading/internal/domain"
	"github.com/Kirk-V/XRPLTrading/internal/xrpl"
)

type fakeClient struct {
	fail  bool
	books int
}

func (f *fakeClient) BookChanges(context.Context) (xrpl.BookChanges, error) {
	if f.fail {
		return xrpl.BookChanges{}, errors.New("endpoint disconnected")
	}
	result := xrpl.BookChanges{LedgerIndex: 42, Validated: true, LedgerTime: uint32(time.Now().Unix() - 946684800)}
	result.Changes = append(result.Changes, struct {
		CurrencyA string `json:"currency_a"`
		CurrencyB string `json:"currency_b"`
	}{"XRP_drops", "rQGbw4Q5jqq2t25r4vjsyRksXvoM1o2Jf6/USD"})
	return result, nil
}
func (f *fakeClient) MarketBook(_ context.Context, pair domain.Pair) (v1.OrderBook, error) {
	f.books++
	if f.fail {
		return v1.OrderBook{}, errors.New("book unavailable")
	}
	return v1.OrderBook{Pair: pair, Bids: []v1.BookLevel{}, Asks: []v1.BookLevel{}}, nil
}

func TestDiscoveryRecoveryAndStaleStatus(t *testing.T) {
	client := &fakeClient{}
	service := New(client, config.Default())
	service.refresh(context.Background())
	service.refresh(context.Background())
	state := service.snapshot()
	if state.Connection != "connected" || len(state.Markets) != 1 || state.LedgerIndex != 42 {
		t.Fatalf("bad state: %+v", state)
	}
	client.fail = true
	service.refresh(context.Background())
	state = service.snapshot()
	if state.Connection != "reconnecting" || state.Error == "" || len(state.Markets) != 1 {
		t.Fatalf("outage lost state: %+v", state)
	}
	client.fail = false
	service.refresh(context.Background())
	if service.snapshot().Error != "" {
		t.Fatal("error survived recovery")
	}
	service.state.LedgerClosedAt = time.Now().Add(-time.Minute)
	if service.snapshot().Connection != "stale" {
		t.Fatal("old ledger marked connected")
	}
}

func TestLocalAPIValidationAndErrors(t *testing.T) {
	client := &fakeClient{}
	service := New(client, config.Default())
	handler := service.Handler(t.TempDir())
	cases := []struct {
		path, host, origin string
		status             int
	}{
		{"/api/v1/state", "localhost:8080", "", 200},
		{"/api/v1/book?currency=USD&issuer=rFake", "localhost:8080", "", 400},
		{"/api/v1/book?currency=USD&issuer=rQGbw4Q5jqq2t25r4vjsyRksXvoM1o2Jf6", "localhost:8080", "", 200},
		{"/api/v1/state", "evil.example", "", 403},
		{"/api/v1/state", "localhost:8080", "https://evil.example", 403},
		{"/api/v1/state", "localhost:8080", "http://localhost:8080", 200},
		{"/api/v1/missing", "localhost:8080", "", 404},
		{"/", "localhost:8080", "", 503},
	}
	for _, test := range cases {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		req.Host = test.host
		req.Header.Set("Origin", test.origin)
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, req)
		if result.Code != test.status {
			t.Errorf("%s: got %d want %d: %s", test.path, result.Code, test.status, result.Body.String())
		}
	}
	if client.books != 1 {
		t.Fatal("invalid request reached upstream")
	}
	client.fail = true
	req := httptest.NewRequest(http.MethodGet, "/api/v1/book?currency=USD&issuer=rQGbw4Q5jqq2t25r4vjsyRksXvoM1o2Jf6", nil)
	req.Host = "localhost:8080"
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if result.Code != 502 || !strings.Contains(result.Body.String(), "book unavailable") {
		t.Fatal("upstream failure was hidden", result)
	}
}

func (f *fakeClient) WatchAMMs(ctx context.Context, emit func(xrpl.AMMStreamUpdate)) error {
	<-ctx.Done()
	return ctx.Err()
}
