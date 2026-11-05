package xrpl

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"

	v1 "github.com/Kirk-V/XRPLTrading/api/v1"
	"github.com/Kirk-V/XRPLTrading/internal/domain"
)

type BookChanges struct {
	LedgerIndex uint32 `json:"ledger_index"`
	LedgerTime  uint32 `json:"ledger_time"`
	Validated   bool   `json:"validated"`
	Changes     []struct {
		CurrencyA string `json:"currency_a"`
		CurrencyB string `json:"currency_b"`
	} `json:"changes"`
}

func (c *APIClient) BookChanges(ctx context.Context) (BookChanges, error) {
	var result BookChanges
	err := c.call(ctx, c.config.JSONRPC, "book_changes", map[string]any{"ledger_index": "validated"}, &result)
	if err == nil && (!result.Validated || result.LedgerIndex == 0) {
		err = fmt.Errorf("endpoint did not return a validated ledger")
	}
	return result, err
}

type wireOffer struct {
	Account    string          `json:"Account"`
	Sequence   uint32          `json:"Sequence"`
	Gets       json.RawMessage `json:"TakerGets"`
	Pays       json.RawMessage `json:"TakerPays"`
	FundedGets json.RawMessage `json:"taker_gets_funded"`
	FundedPays json.RawMessage `json:"taker_pays_funded"`
}

type bookResult struct {
	LedgerIndex uint32      `json:"ledger_index"`
	Validated   *bool       `json:"validated"`
	Offers      []wireOffer `json:"offers"`
}

func (c *APIClient) bookAt(ctx context.Context, gets, pays domain.Asset, ledger any, limit int) (bookResult, error) {
	var result bookResult
	err := c.call(ctx, c.config.JSONRPC, "book_offers", map[string]any{
		"taker_gets": currencyObject(gets), "taker_pays": currencyObject(pays), "ledger_index": ledger, "limit": limit,
	}, &result)
	// book_offers does not guarantee a validated field (Clio omits it).
	// Callers request "validated", then pin the other side to that ledger.
	// An explicit false or missing ledger index must still be rejected.
	if err == nil && ((result.Validated != nil && !*result.Validated) || result.LedgerIndex == 0) {
		err = fmt.Errorf("endpoint did not return a validated book")
	}
	return result, err
}

// MarketBook loads both directions at the same validated ledger. Replacing full
// snapshots avoids implying that raw transaction notifications are book deltas.
func (c *APIClient) MarketBook(ctx context.Context, pair domain.Pair) (v1.OrderBook, error) {
	book := v1.OrderBook{Pair: pair, Bids: []v1.BookLevel{}, Asks: []v1.BookLevel{}}
	if err := pair.Validate(); err != nil {
		return book, err
	}
	if !pair.Base.IsXRP() {
		return book, fmt.Errorf("book base must be XRP")
	}
	asks, err := c.bookAt(ctx, pair.Base, pair.Quote, "validated", 50)
	if err != nil {
		return book, err
	}
	bids, err := c.bookAt(ctx, pair.Quote, pair.Base, asks.LedgerIndex, 50)
	if err != nil {
		return book, err
	}
	if bids.LedgerIndex != asks.LedgerIndex {
		return book, fmt.Errorf("book sides returned different ledgers")
	}
	if book.Asks, err = levels(asks.Offers, pair, false); err != nil {
		return book, err
	}
	if book.Bids, err = levels(bids.Offers, pair, true); err != nil {
		return book, err
	}
	book.LedgerIndex = asks.LedgerIndex
	book.ObservedAt = time.Now().UTC()
	book.Quote = v1.Quote{Pair: pair, LedgerIndex: book.LedgerIndex, ObservedAt: book.ObservedAt}
	if len(book.Asks) > 0 {
		book.Quote.Ask = book.Asks[0].Price
	}
	if len(book.Bids) > 0 {
		book.Quote.Bid = book.Bids[0].Price
	}
	if book.Quote.Bid != "" && book.Quote.Ask != "" {
		bid, _ := new(big.Rat).SetString(book.Quote.Bid)
		ask, _ := new(big.Rat).SetString(book.Quote.Ask)
		book.Quote.Mid = decimal(new(big.Rat).Quo(new(big.Rat).Add(bid, ask), big.NewRat(2, 1)))
		book.Quote.Spread = decimal(new(big.Rat).Sub(ask, bid))
	}
	return book, nil
}

var decimalPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]{1,3})?$`)

func amount(raw json.RawMessage, asset domain.Asset) (*big.Rat, error) {
	var value string
	if asset.IsXRP() {
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("invalid XRP amount")
		}
	} else {
		var token struct {
			Currency string `json:"currency"`
			Issuer   string `json:"issuer"`
			Value    string `json:"value"`
		}
		if err := json.Unmarshal(raw, &token); err != nil {
			return nil, fmt.Errorf("invalid token amount")
		}
		if token.Currency != asset.Currency || token.Issuer != asset.Issuer {
			return nil, fmt.Errorf("offer asset does not match requested market")
		}
		value = token.Value
	}
	if len(value) > 100 || !decimalPattern.MatchString(value) {
		return nil, fmt.Errorf("invalid decimal amount")
	}
	n, ok := new(big.Rat).SetString(value)
	if !ok || n.Sign() < 0 {
		return nil, fmt.Errorf("invalid decimal amount")
	}
	if asset.IsXRP() {
		if !n.IsInt() {
			return nil, fmt.Errorf("XRP drops must be an integer")
		}
		n.Quo(n, big.NewRat(1000000, 1))
	}
	return n, nil
}

// Display values use 16 significant decimal digits without float arithmetic.
func decimal(n *big.Rat) string { return new(big.Float).SetPrec(256).SetRat(n).Text('g', 16) }

func levels(offers []wireOffer, pair domain.Pair, bid bool) ([]v1.BookLevel, error) {
	result := make([]v1.BookLevel, 0, len(offers))
	for _, offer := range offers {
		getsAsset, paysAsset := pair.Base, pair.Quote
		if bid {
			getsAsset, paysAsset = paysAsset, getsAsset
		}
		gets, err := amount(offer.Gets, getsAsset)
		if err != nil {
			return nil, err
		}
		pays, err := amount(offer.Pays, paysAsset)
		if err != nil {
			return nil, err
		}
		if gets.Sign() == 0 || pays.Sign() == 0 {
			continue
		}
		xrp, token := gets, pays
		if bid {
			xrp, token = pays, gets
		}
		price := new(big.Rat).Quo(token, xrp)
		fraction := big.NewRat(1, 1)
		for _, funded := range []struct {
			raw   json.RawMessage
			asset domain.Asset
			total *big.Rat
		}{{offer.FundedGets, getsAsset, gets}, {offer.FundedPays, paysAsset, pays}} {
			if len(funded.raw) == 0 || string(funded.raw) == "null" {
				continue
			}
			n, err := amount(funded.raw, funded.asset)
			if err != nil {
				return nil, err
			}
			ratio := new(big.Rat).Quo(n, funded.total)
			if ratio.Cmp(fraction) < 0 {
				fraction = ratio
			}
		}
		size := new(big.Rat).Mul(xrp, fraction)
		if size.Sign() == 0 {
			continue
		}
		result = append(result, v1.BookLevel{Price: decimal(price), Size: decimal(size), Owner: offer.Account, Sequence: offer.Sequence})
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, _ := new(big.Rat).SetString(result[i].Price)
		b, _ := new(big.Rat).SetString(result[j].Price)
		if bid {
			return a.Cmp(b) > 0
		}
		return a.Cmp(b) < 0
	})
	return result, nil
}

// ValidateMarketInput bounds user input before it reaches the upstream API.
func ValidateMarketInput(currency, issuer string) (domain.Pair, error) {
	currency = strings.TrimSpace(currency)
	issuer = strings.TrimSpace(issuer)
	validCurrency := regexp.MustCompile(`^(?:[A-Za-z0-9?!@#$%^&*<>(){}\[\]|]{3}|[A-Fa-f0-9]{40})$`)
	validIssuer := regexp.MustCompile(`^r[1-9A-HJ-NP-Za-km-z]{24,34}$`)
	if !validCurrency.MatchString(currency) || !validIssuer.MatchString(issuer) {
		return domain.Pair{}, fmt.Errorf("enter a 3-character currency or 40-character hex code and a classic issuer address")
	}
	// Standard currency codes are case-sensitive on the ledger.
	if len(currency) == 40 {
		currency = strings.ToUpper(currency)
	}
	return domain.NewXRPTokenPair(domain.Asset{Currency: currency, Issuer: issuer})
}
