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
	}
	return nil
}
