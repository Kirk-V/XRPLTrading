package server

import (
	"context"
	"time"

	v1 "github.com/Kirk-V/XRPLTrading/api/v1"
	"github.com/Kirk-V/XRPLTrading/internal/market"
	"github.com/Kirk-V/XRPLTrading/internal/xrpl"
)

func (s *Service) runAMMs(ctx context.Context) {
	backoff := 250 * time.Millisecond
	for ctx.Err() == nil {
		started := time.Now()
		err := s.client.WatchAMMs(ctx, s.onAMMUpdate)
		if ctx.Err() != nil {
			return
		}
		s.mu.Lock()
		s.state.AMMDiscovery.Connection = "reconnecting"
		s.state.AMMDiscovery.Interrupted = true
		s.state.AMMDiscovery.Error = "AMM stream disconnected; retrying."
		if err != nil {
			s.state.AMMDiscovery.Error = err.Error()
		}
		s.mu.Unlock()
		if time.Since(started) > 30*time.Second {
			backoff = 250 * time.Millisecond
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(backoff*2, 5*time.Second)
	}
}

func (s *Service) onAMMUpdate(update xrpl.AMMStreamUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := &s.state.AMMDiscovery
	status.Connection = "connected"
	status.Error = ""
	if update.Creation == nil {
		status.LastMessageAt = time.Now().UTC()
		if update.LedgerIndex > 0 {
			if status.LedgerIndex > 0 && update.LedgerIndex > status.LedgerIndex+1 {
				status.Interrupted = true
			}
			status.LedgerIndex = update.LedgerIndex
		}
		return
	}
	creation := *update.Creation
	for _, previous := range s.state.AMMCreations {
		if previous.TransactionHash == creation.TransactionHash {
			return
		}
	}
	// Keep creation notifications even when the pair was already discovered via trades.
	s.state.AMMCreations = append([]v1.AMMCreation{creation}, s.state.AMMCreations...)
	if len(s.state.AMMCreations) > 100 {
		s.state.AMMCreations = s.state.AMMCreations[:100]
	}
	for _, pair := range s.state.Markets {
		if pair == creation.Pair {
			return
		}
	}
	if len(s.state.Markets) >= 200 {
		evicted := s.state.Markets[0]
		s.state.Markets = s.state.Markets[1:]
		delete(s.ammChecked, evicted)
		for i, item := range s.state.AMMStatuses {
			if item.Pair == evicted {
				s.state.AMMStatuses = append(s.state.AMMStatuses[:i], s.state.AMMStatuses[i+1:]...)
				break
			}
		}
		// Rebuild the bounded deduplication set after evicting an old market, so
		// polling can discover it again later and fresh AMMs are never hidden by a cap.
		s.discovery = market.NewPairDiscovery()
		for _, pair := range s.state.Markets {
			_, _, _ = s.discovery.Observe("XRP_drops", pair.Quote.Issuer+"/"+pair.Quote.Currency)
		}
	}
	_, _, _ = s.discovery.Observe("XRP_drops", creation.Pair.Quote.Issuer+"/"+creation.Pair.Quote.Currency)
	s.state.Markets = append(s.state.Markets, creation.Pair)
	s.ensureAMMStatusLocked(creation.Pair)
}
