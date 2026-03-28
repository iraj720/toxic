package market

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type Quote struct {
	BaseSymbol  string
	QuoteSymbol string
	Price       decimal.Decimal
	Source      string
	FetchedAt   time.Time
}

type PriceProvider interface {
	GetPrice(ctx context.Context, baseSymbol, quoteSymbol string) (Quote, error)
}
