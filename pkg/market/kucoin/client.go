package kucoin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"exchange/pkg/market"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) GetPrice(ctx context.Context, baseSymbol, quoteSymbol string) (market.Quote, error) {
	symbol := strings.ToUpper(strings.TrimSpace(baseSymbol)) + "-" + strings.ToUpper(strings.TrimSpace(quoteSymbol))

	endpoint, err := url.Parse(c.baseURL + "/api/v1/market/orderbook/level1")
	if err != nil {
		return market.Quote{}, err
	}
	query := endpoint.Query()
	query.Set("symbol", symbol)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return market.Quote{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return market.Quote{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return market.Quote{}, fmt.Errorf("kucoin returned status %d", resp.StatusCode)
	}

	var payload struct {
		Code string `json:"code"`
		Data struct {
			Price string `json:"price"`
			Time  int64  `json:"time"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return market.Quote{}, err
	}
	if payload.Code != "200000" {
		return market.Quote{}, fmt.Errorf("kucoin returned code %s", payload.Code)
	}
	price, err := decimal.NewFromString(payload.Data.Price)
	if err != nil {
		return market.Quote{}, fmt.Errorf("parse kucoin price: %w", err)
	}

	fetchedAt := time.Now().UTC()
	if payload.Data.Time > 0 {
		fetchedAt = time.UnixMilli(payload.Data.Time).UTC()
	}

	return market.Quote{
		BaseSymbol:  strings.ToUpper(baseSymbol),
		QuoteSymbol: strings.ToUpper(quoteSymbol),
		Price:       price,
		Source:      "kucoin",
		FetchedAt:   fetchedAt,
	}, nil
}
