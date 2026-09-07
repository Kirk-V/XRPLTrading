package v1

import (
	"testing"

	"github.com/Kirk-V/XRPLTrading/internal/domain"
)

func TestReadModelsKeepExactAmountsAsStrings(t *testing.T) {
	balance := Balance{
		WalletID:  "wallet-1",
		Asset:     domain.XRP(),
		Total:     "123456789.123456",
		Spendable: "123456789.123400",
	}
	if err := balance.Validate(); err != nil {
		t.Fatal(err)
	}
	if balance.Total != "123456789.123456" {
		t.Fatalf("amount was transformed: %q", balance.Total)
	}
}

func TestOrderValidationRequiresExplicitSideAndPair(t *testing.T) {
	order := Order{ID: "order-1", WalletID: "wallet-1", Amount: "10", Status: OrderSubmitted}
	if err := order.Validate(); err == nil {
		t.Fatal("expected incomplete order to be rejected")
	}
	order.Pair, _ = domain.NewXRPTokenPair(domain.Issued("USD", "rIssuer"))
	order.Side = "buy"
	if err := order.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionStatusesAreExplicit(t *testing.T) {
	transaction := Transaction{ID: "tx-1", WalletID: "wallet-1", Status: TransactionValidated}
	if err := transaction.Validate(); err != nil {
		t.Fatal(err)
	}
}
