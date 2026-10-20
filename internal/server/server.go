// Package server exposes the read-only local application API and built web UI.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	v1 "github.com/Kirk-V/XRPLTrading/api/v1"
	"github.com/Kirk-V/XRPLTrading/internal/config"
	"github.com/Kirk-V/XRPLTrading/internal/domain"
	"github.com/Kirk-V/XRPLTrading/internal/market"
	"github.com/Kirk-V/XRPLTrading/internal/xrpl"
)

type Client interface {
	BookChanges(context.Context) (xrpl.BookChanges, error)
	MarketBook(context.Context, domain.Pair) (v1.OrderBook, error)
	WatchAMMs(context.Context, func(xrpl.AMMStreamUpdate)) error
	AMMStatus(context.Context, domain.Pair) (v1.AMMStatus, error)
}

type Service struct {
	client      Client
	mu          sync.RWMutex
	state       v1.MarketState
	discovery   *market.PairDiscovery
	ammChecked  map[domain.Pair]time.Time
	ammFailures uint64
}

func New(client Client, cfg config.Config) *Service {
	return &Service{client: client, discovery: market.NewPairDiscovery(), ammChecked: make(map[domain.Pair]time.Time), state: v1.MarketState{
		Network: string(cfg.Network), ReadOnly: true, Endpoint: cfg.JSONRPC, Connection: "connecting", Markets: []domain.Pair{},
		AMMDiscovery: v1.AMMDiscovery{Connection: "connecting"}, AMMCreations: []v1.AMMCreation{}, AMMStatuses: []v1.AMMStatus{},
	}}
}

// Run samples the latest validated ledger. This is discovery, not a complete
// ledger history: intermediate ledgers can be skipped during polling/outages.
func (s *Service) Run(ctx context.Context) {
	ammDone := make(chan struct{})
	go func() { defer close(ammDone); s.runAMMs(ctx) }()
	statusDone := make(chan struct{})
	go func() { defer close(statusDone); s.runAMMStatusChecks(ctx) }()
	defer func() { <-ammDone; <-statusDone }()
	for {
		s.refresh(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (s *Service) refresh(ctx context.Context) {
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	changes, err := s.client.BookChanges(requestCtx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.state.Connection = "reconnecting"
		s.state.Error = err.Error()
		return
	}
	s.state.Connection = "connected"
	s.state.Error = ""
	s.state.LedgerIndex = changes.LedgerIndex
	s.state.LedgerClosedAt = time.Unix(int64(changes.LedgerTime)+946684800, 0).UTC()
	s.state.ObservedAt = time.Now().UTC()
	for _, change := range changes.Changes {
		if len(s.state.Markets) >= 200 {
			break
		}
		pair, fresh, err := s.discovery.Observe(change.CurrencyA, change.CurrencyB)
		if err != nil || !fresh {
			continue
		}
		s.state.Markets = append(s.state.Markets, pair)
		s.ensureAMMStatusLocked(pair)
	}
}

func (s *Service) snapshot() v1.MarketState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := s.state
	result.Markets = append([]domain.Pair{}, s.state.Markets...)
	result.AMMCreations = append([]v1.AMMCreation{}, s.state.AMMCreations...)
	result.AMMStatuses = append([]v1.AMMStatus{}, s.state.AMMStatuses...)
	if result.AMMDiscovery.Connection == "connected" && time.Since(result.AMMDiscovery.LastMessageAt) > 30*time.Second {
		result.AMMDiscovery.Connection = "stale"
		result.AMMDiscovery.Error = "Waiting for AMM stream ledger heartbeat."
	}
	if result.Connection == "connected" && (time.Since(result.ObservedAt) > 20*time.Second || time.Since(result.LedgerClosedAt) > 30*time.Second) {
		result.Connection = "stale"
		result.Error = "Waiting for a recent validated ledger."
	}
	return result
}

func (s *Service) Handler(webDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/state", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, s.snapshot()) })
	// Bound concurrent upstream work from polling browser clients.
	bookSlots := make(chan struct{}, 4)
	mux.HandleFunc("GET /api/v1/book", func(w http.ResponseWriter, r *http.Request) {
		pair, err := xrpl.ValidateMarketInput(r.URL.Query().Get("currency"), r.URL.Query().Get("issuer"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		select {
		case bookSlots <- struct{}{}:
			defer func() { <-bookSlots }()
		default:
			writeError(w, http.StatusTooManyRequests, fmt.Errorf("busy loading books; retry shortly"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		book, err := s.client.MarketBook(ctx, pair)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, book)
	})
	mux.HandleFunc("GET /api/v1/amm", func(w http.ResponseWriter, r *http.Request) {
		pair, err := xrpl.ValidateMarketInput(r.URL.Query().Get("currency"), r.URL.Query().Get("issuer"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		result, err := s.client.AMMStatus(ctx, pair)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown API route"))
	})
	files := http.FileServer(http.Dir(webDir))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
			http.Error(w, "Build the UI first: npm --prefix web ci && npm --prefix web run build. For development run npm --prefix web run dev.", http.StatusServiceUnavailable)
			return
		}
		files.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "local access only", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
				http.Error(w, "same-origin access required", http.StatusForbidden)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
