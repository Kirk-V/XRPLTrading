package server

import (
	"context"
	"time"

	v1 "github.com/Kirk-V/XRPLTrading/api/v1"
	"github.com/Kirk-V/XRPLTrading/internal/domain"
)

const ammStatusTTL = 5 * time.Minute
const ammStatusRetry = 30 * time.Second

func (s *Service) ensureAMMStatusLocked(pair domain.Pair) {
	for _, item := range s.state.AMMStatuses {
		if item.Pair == pair {
			return
		}
	}
	s.state.AMMStatuses = append(s.state.AMMStatuses, v1.AMMStatus{Pair: pair, State: "unknown"})
}

func (s *Service) nextAMMStatusPair() (domain.Pair, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, pair := range s.state.Markets {
		s.ensureAMMStatusLocked(pair)
		if now.Sub(s.ammChecked[pair]) >= ammStatusTTL {
			s.ammChecked[pair] = now
			return pair, true
		}
	}
	return domain.Pair{}, false
}

// Status checks are paced to protect the public endpoint. A failed lookup
// remains unknown and retries later; it is never displayed as "no pool".
func (s *Service) runAMMStatusChecks(ctx context.Context) {
	for ctx.Err() == nil {
		pair, found := s.nextAMMStatusPair()
		if found {
			requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			result, err := s.client.AMMStatus(requestCtx, pair)
			cancel()
			s.mu.Lock()
			if err != nil {
				result = v1.AMMStatus{Pair: pair, State: "unknown"}
				s.ammChecked[pair] = time.Now().Add(-(ammStatusTTL - ammStatusRetry))
				s.ammFailures++
			}
			for i := range s.state.AMMStatuses {
				if s.state.AMMStatuses[i].Pair == pair {
					s.state.AMMStatuses[i] = result
					break
				}
			}
			s.mu.Unlock()
		}
		delay := 2 * time.Second
		if found {
			delay = 500 * time.Millisecond
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
