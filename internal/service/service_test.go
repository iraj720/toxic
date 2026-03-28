package service

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"exchange/internal/config"
	"exchange/internal/domain"
	"exchange/pkg/market"
)

type fakeStore struct {
	overrideRate decimal.Decimal
	hasRate      bool
}

func (f fakeStore) EnsureUser(context.Context, domain.TelegramProfile) (domain.User, bool, error) {
	return domain.User{}, false, nil
}

func (f fakeStore) GetUserByTelegramID(context.Context, int64) (domain.User, error) {
	return domain.User{ID: 1, TelegramUserID: 10, Locale: "fa"}, nil
}

func (f fakeStore) GetDashboard(context.Context, int64, []string) (domain.Dashboard, error) {
	return domain.Dashboard{}, nil
}

func (f fakeStore) SetUserLocale(context.Context, int64, string) error {
	return nil
}

func (f fakeStore) AddContactByShareCode(context.Context, int64, string) (domain.Contact, error) {
	return domain.Contact{}, nil
}

func (f fakeStore) ResolveRecipient(context.Context, int64, string) (domain.User, error) {
	return domain.User{}, nil
}

func (f fakeStore) Deposit(context.Context, int64, string, decimal.Decimal) (domain.Transaction, error) {
	return domain.Transaction{}, nil
}

func (f fakeStore) CreatePendingDeposit(context.Context, int64, string, decimal.Decimal, string, string) (domain.Transaction, error) {
	return domain.Transaction{}, nil
}

func (f fakeStore) ListPendingDeposits(context.Context, int) ([]domain.PendingDeposit, error) {
	return nil, nil
}

func (f fakeStore) ApprovePendingDeposit(context.Context, string, string) (domain.Transaction, error) {
	return domain.Transaction{}, nil
}

func (f fakeStore) Buy(context.Context, int64, domain.TradeQuote) (domain.Transaction, error) {
	return domain.Transaction{}, nil
}

func (f fakeStore) Sell(context.Context, int64, domain.TradeQuote) (domain.Transaction, error) {
	return domain.Transaction{}, nil
}

func (f fakeStore) Transfer(context.Context, int64, int64, string, decimal.Decimal) (domain.Transaction, error) {
	return domain.Transaction{}, nil
}

func (f fakeStore) GetQuoteToSettlementRate(context.Context) (decimal.Decimal, bool, error) {
	return f.overrideRate, f.hasRate, nil
}

func (f fakeStore) SetQuoteToSettlementRate(context.Context, decimal.Decimal) error {
	return nil
}

type fakeProvider struct {
	price decimal.Decimal
}

func (f fakeProvider) GetPrice(context.Context, string, string) (market.Quote, error) {
	return market.Quote{
		BaseSymbol:  "BTC",
		QuoteSymbol: "USDT",
		Price:       f.price,
		Source:      "fake",
		FetchedAt:   time.Now().UTC(),
	}, nil
}

func TestQuoteBuyUsesSettlementRate(t *testing.T) {
	cfg := config.Config{
		Telegram: config.TelegramConfig{BotToken: "token"},
		Database: config.DatabaseConfig{Address: "postgres://localhost/exchange"},
		Market: config.MarketConfig{
			Provider:              "kucoin",
			QuoteCurrency:         "USDT",
			SettlementCurrency:    "TMN",
			QuoteToSettlementRate: "2",
			Coins: []config.CoinConfig{
				{Symbol: "BTC"},
			},
		},
	}
	cfg.EnabledCoins()

	svc := New(cfg, fakeStore{}, fakeProvider{price: decimal.NewFromInt(100)})
	quote, err := svc.QuoteBuy(context.Background(), "BTC", decimal.NewFromInt(200))
	if err != nil {
		t.Fatalf("QuoteBuy returned error: %v", err)
	}

	if !quote.PriceInSettlement.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("expected settlement price 200, got %s", quote.PriceInSettlement)
	}
	if !quote.AssetAmount.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("expected asset amount 1, got %s", quote.AssetAmount)
	}
}

func TestCurrentQuoteToSettlementRateFallsBackToConfig(t *testing.T) {
	cfg := config.Config{
		Telegram: config.TelegramConfig{BotToken: "token"},
		Database: config.DatabaseConfig{Address: "postgres://localhost/exchange"},
		Market: config.MarketConfig{
			QuoteCurrency:         "USDT",
			SettlementCurrency:    "TMN",
			QuoteToSettlementRate: "42000",
			Coins: []config.CoinConfig{
				{Symbol: "BTC"},
			},
		},
	}

	svc := New(cfg, fakeStore{}, fakeProvider{price: decimal.NewFromInt(100)})
	rate, err := svc.CurrentQuoteToSettlementRate(context.Background())
	if err != nil {
		t.Fatalf("CurrentQuoteToSettlementRate returned error: %v", err)
	}

	if rate.Source != "config" {
		t.Fatalf("expected config source, got %q", rate.Source)
	}
	if !rate.Rate.Equal(decimal.NewFromInt(42000)) {
		t.Fatalf("expected config rate 42000, got %s", rate.Rate)
	}
}

func TestCurrentQuoteToSettlementRateUsesAdminOverride(t *testing.T) {
	cfg := config.Config{
		Telegram: config.TelegramConfig{BotToken: "token"},
		Database: config.DatabaseConfig{Address: "postgres://localhost/exchange"},
		Market: config.MarketConfig{
			QuoteCurrency:         "USDT",
			SettlementCurrency:    "TMN",
			QuoteToSettlementRate: "42000",
			Coins: []config.CoinConfig{
				{Symbol: "BTC"},
			},
		},
	}

	svc := New(cfg, fakeStore{
		overrideRate: decimal.NewFromInt(51000),
		hasRate:      true,
	}, fakeProvider{price: decimal.NewFromInt(100)})

	rate, err := svc.CurrentQuoteToSettlementRate(context.Background())
	if err != nil {
		t.Fatalf("CurrentQuoteToSettlementRate returned error: %v", err)
	}

	if rate.Source != "admin" {
		t.Fatalf("expected admin source, got %q", rate.Source)
	}
	if !rate.Rate.Equal(decimal.NewFromInt(51000)) {
		t.Fatalf("expected override rate 51000, got %s", rate.Rate)
	}
}
