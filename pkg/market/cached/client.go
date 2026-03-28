package cached

import (
	"context"
	"strings"
	"sync"
	"time"

	"exchange/pkg/market"
)

type Client struct {
	inner market.PriceProvider
	ttl   time.Duration

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	quote     market.Quote
	expiresAt time.Time
}

func New(inner market.PriceProvider, ttl time.Duration) *Client {
	return &Client{
		inner: inner,
		ttl:   ttl,
		cache: map[string]cacheEntry{},
	}
}

func (c *Client) GetPrice(ctx context.Context, baseSymbol, quoteSymbol string) (market.Quote, error) {
	key := strings.ToUpper(strings.TrimSpace(baseSymbol)) + ":" + strings.ToUpper(strings.TrimSpace(quoteSymbol))
	now := time.Now().UTC()

	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.quote, nil
	}

	quote, err := c.inner.GetPrice(ctx, baseSymbol, quoteSymbol)
	if err != nil {
		return market.Quote{}, err
	}

	c.mu.Lock()
	c.cache[key] = cacheEntry{
		quote:     quote,
		expiresAt: now.Add(c.ttl),
	}
	c.mu.Unlock()

	return quote, nil
}
