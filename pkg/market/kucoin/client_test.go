package kucoin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/market/orderbook/level1" {
			t.Fatalf("unexpected path %s", got)
		}
		if got := r.URL.Query().Get("symbol"); got != "BTC-USDT" {
			t.Fatalf("unexpected symbol %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"200000","data":{"price":"50000.1","time":1700000000000}}`))
	}))
	defer server.Close()

	client := New(server.URL, time.Second)
	quote, err := client.GetPrice(context.Background(), "btc", "usdt")
	if err != nil {
		t.Fatalf("GetPrice returned error: %v", err)
	}
	if quote.Price.String() != "50000.1" {
		t.Fatalf("expected 50000.1, got %s", quote.Price)
	}
	if quote.Source != "kucoin" {
		t.Fatalf("expected source kucoin, got %s", quote.Source)
	}
}
