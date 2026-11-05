package config

import (
	"errors"
	"fmt"
	"net/url"
)

type Network string

const (
	Testnet Network = "testnet"
	Mainnet Network = "mainnet"
)

type Config struct {
	Network   Network `json:"network"`
	WebSocket string  `json:"websocket"`
	JSONRPC   string  `json:"jsonrpc"`
	ReadOnly  bool    `json:"read_only"`
}

func Default() Config {
	return Config{
		Network:   Testnet,
		WebSocket: "wss://s.altnet.rippletest.net:51233",
		JSONRPC:   "https://s.altnet.rippletest.net:51234",
		ReadOnly:  true,
	}
}

// ForNetwork selects matching HTTP and WebSocket endpoints together. All
// supported configurations remain read-only, including explicit mainnet use.
func ForNetwork(network Network) (Config, error) {
	cfg := Default()
	switch network {
	case Testnet:
	case Mainnet:
		cfg.Network = Mainnet
		cfg.JSONRPC = "https://s1.ripple.com:51234/"
		cfg.WebSocket = "wss://s1.ripple.com/"
	default:
		return Config{}, fmt.Errorf("unsupported network %q; use testnet or mainnet", network)
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.Network != Testnet && c.Network != Mainnet {
		return fmt.Errorf("unsupported network %q", c.Network)
	}
	if c.Network == Mainnet && c.ReadOnly == false {
		return errors.New("mainnet trading is not enabled by the foundation scaffold")
	}
	for name, endpoint := range map[string]string{"websocket": c.WebSocket, "jsonrpc": c.JSONRPC} {
		u, err := url.Parse(endpoint)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("invalid %s endpoint", name)
		}
		if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("%s endpoint must not contain credentials, a query, or a fragment", name)
		}
		if name == "jsonrpc" && u.Scheme != "http" && u.Scheme != "https" {
			return errors.New("JSON-RPC endpoint must use HTTP or HTTPS")
		}
		if name == "websocket" && u.Scheme != "ws" && u.Scheme != "wss" {
			return errors.New("WebSocket endpoint must use WS or WSS")
		}
	}
	return nil
}
