package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"exchange/internal/config"
	"exchange/internal/localization"
	"exchange/internal/service"
	"exchange/internal/store"
	"exchange/internal/telegram"
	"exchange/pkg/market"
	"exchange/pkg/market/cached"
	"exchange/pkg/market/kucoin"
)

func Run(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pgStore, err := store.New(ctx, cfg.Database.Address, cfg.Market.SettlementCurrency, cfg.CoinSymbols())
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pgStore.Close()

	if err := pgStore.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	var priceProvider market.PriceProvider
	switch cfg.Market.Provider {
	case "kucoin":
		priceProvider = cached.New(
			kucoin.New(cfg.Providers.Kucoin.BaseURL, time.Duration(cfg.Providers.Kucoin.TimeoutSeconds)*time.Second),
			20*time.Second,
		)
	default:
		return fmt.Errorf("unsupported market provider %q", cfg.Market.Provider)
	}

	botService := service.New(cfg, pgStore, priceProvider)
	tgClient := telegram.NewClient(cfg.Telegram.BaseURL, cfg.Telegram.BotToken)
	bot := telegram.NewBot(log.Default(), cfg, tgClient, botService, localization.New())

	return bot.Run(ctx)
}
