package market

import (
	"errors"
	"strings"

	"github.com/Kirk-V/XRPLTrading/internal/domain"
)

// PairDiscovery turns XRPL book-change asset labels into canonical pairs.
// It intentionally reports observed markets only; it does not assess safety,
// liquidity, or whether a token is legitimate.
type PairDiscovery struct {
	seen map[string]domain.Pair
}

func NewPairDiscovery() *PairDiscovery { return &PairDiscovery{seen: make(map[string]domain.Pair)} }

func (d *PairDiscovery) Observe(currencyA, currencyB string) (domain.Pair, bool, error) {
	a, err := parseAssetLabel(currencyA)
	if err != nil {
		return domain.Pair{}, false, err
	}
	b, err := parseAssetLabel(currencyB)
	if err != nil {
		return domain.Pair{}, false, err
	}
	var pair domain.Pair
	if a.IsXRP() {
		pair, err = domain.NewXRPTokenPair(b)
	} else if b.IsXRP() {
		pair, err = domain.NewXRPTokenPair(a)
	} else {
		return domain.Pair{}, false, errors.New("pair is not an XRP/token market")
	}
	if err != nil {
		return domain.Pair{}, false, err
	}
	key := pair.Base.Currency + ":" + pair.Quote.Currency + ":" + pair.Quote.Issuer
	if _, exists := d.seen[key]; exists {
		return pair, false, nil
	}
	d.seen[key] = pair
	return pair, true, nil
}

func parseAssetLabel(label string) (domain.Asset, error) {
	label = strings.TrimSpace(label)
	if label == "XRP_drops" || label == "XRP" {
		return domain.XRP(), nil
	}
	parts := strings.SplitN(label, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return domain.Asset{}, errors.New("invalid XRPL book asset label")
	}
	return domain.Issued(parts[1], parts[0]), nil
}
