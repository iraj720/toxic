package cached

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"exchange/pkg/market"
)

type fakeProvider struct {
	calls int
}

func (f *fakeProvider) GetPrice(context.Context, string, string) (market.Quote, error) {
	f.calls++
	return market.Quote{
		BaseSymbol:  "BTC",
		QuoteSymbol: "USDT",
		Price:       decimal.NewFromInt(10),
		Source:      "fake",
		FetchedAt:   time.Now().UTC(),
	}, nil
}

func TestGetPriceUsesCache(t *testing.T) {
	provider := &fakeProvider{}
	client := New(provider, 20*time.Second)

	if _, err := client.GetPrice(context.Background(), "BTC", "USDT"); err != nil {
		t.Fatalf("GetPrice returned error: %v", err)
	}
	if _, err := client.GetPrice(context.Background(), "BTC", "USDT"); err != nil {
		t.Fatalf("GetPrice returned error on cache hit: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("expected provider to be called once, got %d", provider.calls)
	}
}
