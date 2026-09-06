package domain

import "testing"

func TestNewXRPTokenPairCanonicalizesIdentity(t *testing.T) {
	pair, err := NewXRPTokenPair(Issued(" usd ", "rIssuer"))
	if err != nil {
		t.Fatal(err)
	}
	if !pair.Base.IsXRP() || pair.Quote.Currency != "USD" || pair.Quote.Issuer != "rIssuer" {
		t.Fatalf("unexpected pair: %+v", pair)
	}
}

func TestAssetRejectsAmbiguousXRP(t *testing.T) {
	if err := (Asset{Currency: "XRP", Issuer: "rIssuer"}).Validate(); err == nil {
		t.Fatal("expected XRP issuer validation error")
	}
}

func TestPairRejectsTwoIssuedAssetsForInitialScope(t *testing.T) {
	pair := Pair{Base: Issued("USD", "rOne"), Quote: Issued("EUR", "rTwo")}
	if err := pair.Validate(); err == nil {
		t.Fatal("expected token/token pair to be rejected for initial scope")
	}
}
