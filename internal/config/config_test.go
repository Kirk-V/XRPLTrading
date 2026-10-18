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

func TestNetworkEndpointsStayTogetherAndReadOnly(t *testing.T) {
	for _, test := range []struct {
		network Network
		rpc, ws string
	}{
		{Testnet, "https://s.altnet.rippletest.net:51234", "wss://s.altnet.rippletest.net:51233"},
		{Mainnet, "https://s1.ripple.com:51234/", "wss://s1.ripple.com/"},
	} {
		cfg, err := ForNetwork(test.network)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Network != test.network || cfg.JSONRPC != test.rpc || cfg.WebSocket != test.ws || !cfg.ReadOnly {
			t.Fatalf("mismatched network configuration: %+v", cfg)
		}
	}
	if _, err := ForNetwork("unknown"); err == nil {
		t.Fatal("unknown network silently accepted")
	}
}
