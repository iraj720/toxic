package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := `
telegram:
  bot_token: "token"
database:
  address: "postgres://localhost/exchange"
market:
  usdt_to_tmn_rate: "56000"
  coins:
    - symbol: btc
`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Telegram.PollTimeoutSeconds != 30 {
		t.Fatalf("expected default poll timeout, got %d", cfg.Telegram.PollTimeoutSeconds)
	}
	if got := cfg.EnabledCoins()[0].Symbol; got != "BTC" {
		t.Fatalf("expected normalized coin symbol BTC, got %s", got)
	}
	if got := cfg.QuoteToSettlementDecimal().String(); got != "56000" {
		t.Fatalf("expected aliased quote-to-settlement rate 56000, got %s", got)
	}
}
