package market

import "testing"

func TestPairDiscoveryDeduplicatesDirection(t *testing.T) {
	d := NewPairDiscovery()
	if _, fresh, err := d.Observe("XRP_drops", "rIssuer/USD"); err != nil || !fresh {
		t.Fatalf("first observation: fresh=%v err=%v", fresh, err)
	}
	if _, fresh, err := d.Observe("rIssuer/USD", "XRP_drops"); err != nil || fresh {
		t.Fatalf("reverse observation: fresh=%v err=%v", fresh, err)
	}
}

func TestPairDiscoveryRejectsTokenToken(t *testing.T) {
	if _, _, err := NewPairDiscovery().Observe("rOne/USD", "rTwo/EUR"); err == nil {
		t.Fatal("expected token/token market to be outside initial scope")
	}
}
