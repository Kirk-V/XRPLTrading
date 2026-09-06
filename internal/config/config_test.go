package config

import "testing"

func TestDefaultIsSafe(t *testing.T) {
	c := Default()
	if c.Network != Testnet || !c.ReadOnly {
		t.Fatalf("unsafe default: %+v", c)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMainnetTradingIsRejectedByScaffold(t *testing.T) {
	c := Default()
	c.Network = Mainnet
	c.ReadOnly = false
	if err := c.Validate(); err == nil {
		t.Fatal("expected mainnet trading to be rejected")
	}
}
