package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Asset identifies an XRPL asset without relying on a ticker symbol. Issued
// currencies are uniquely identified by currency code and issuer address.
type Asset struct {
	Currency string `json:"currency"`
	Issuer   string `json:"issuer,omitempty"`
}

func XRP() Asset { return Asset{Currency: "XRP"} }

func Issued(currency, issuer string) Asset {
	return Asset{Currency: strings.ToUpper(strings.TrimSpace(currency)), Issuer: strings.TrimSpace(issuer)}
}

func (a Asset) IsXRP() bool { return strings.EqualFold(a.Currency, "XRP") && a.Issuer == "" }

func (a Asset) Validate() error {
	if strings.TrimSpace(a.Currency) == "" {
		return errors.New("asset currency is required")
	}
	if a.IsXRP() {
		return nil
	}
	if strings.EqualFold(a.Currency, "XRP") {
		return errors.New("XRP must not have an issuer")
	}
	if strings.TrimSpace(a.Issuer) == "" {
		return fmt.Errorf("issuer is required for issued currency %q", a.Currency)
	}
	return nil
}

// Pair is stored in a canonical orientation so XRP/token and token/XRP
// observations resolve to the same market.
type Pair struct {
	Base  Asset `json:"base"`
	Quote Asset `json:"quote"`
}

func NewXRPTokenPair(token Asset) (Pair, error) {
	if err := token.Validate(); err != nil {
		return Pair{}, err
	}
	if token.IsXRP() {
		return Pair{}, errors.New("pair token must be an issued currency")
	}
	return Pair{Base: XRP(), Quote: token}, nil
}

func (p Pair) Validate() error {
	if err := p.Base.Validate(); err != nil {
		return fmt.Errorf("base asset: %w", err)
	}
	if err := p.Quote.Validate(); err != nil {
		return fmt.Errorf("quote asset: %w", err)
	}
	if p.Base.IsXRP() == p.Quote.IsXRP() {
		return errors.New("initial pair support requires exactly one XRP asset")
	}
	return nil
}
