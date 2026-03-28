package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App       AppConfig       `yaml:"app"`
	Admin     AdminConfig     `yaml:"admin"`
	Telegram  TelegramConfig  `yaml:"telegram"`
	Database  DatabaseConfig  `yaml:"database"`
	Deposit   DepositConfig   `yaml:"deposit"`
	Fees      FeesConfig      `yaml:"fees"`
	Market    MarketConfig    `yaml:"market"`
	Providers ProvidersConfig `yaml:"providers"`
}

type AppConfig struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"`
}

type AdminConfig struct {
	AuthCode string `yaml:"auth_code"`
}

type TelegramConfig struct {
	BotToken           string `yaml:"bot_token"`
	BaseURL            string `yaml:"base_url"`
	PollTimeoutSeconds int    `yaml:"poll_timeout_seconds"`
}

type DatabaseConfig struct {
	Address string `yaml:"address"`
}

type DepositConfig struct {
	CardNumber string `yaml:"card_number"`
}

type FeesConfig struct {
	TransactionPercent string `yaml:"transaction_percent"`
}

type MarketConfig struct {
	Provider              string       `yaml:"provider"`
	QuoteCurrency         string       `yaml:"quote_currency"`
	SettlementCurrency    string       `yaml:"settlement_currency"`
	QuoteToSettlementRate string       `yaml:"quote_to_settlement_rate"`
	USDTToTMNRate         string       `yaml:"usdt_to_tmn_rate"`
	Coins                 []CoinConfig `yaml:"coins"`
}

type CoinConfig struct {
	Symbol  string `yaml:"symbol"`
	Name    string `yaml:"name"`
	Enabled *bool  `yaml:"enabled"`
}

func (c CoinConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

type ProvidersConfig struct {
	Kucoin KucoinConfig `yaml:"kucoin"`
}

type KucoinConfig struct {
	BaseURL        string `yaml:"base_url"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode yaml: %w", err)
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.App.Name == "" {
		c.App.Name = "exchange-bot"
	}
	if c.App.Env == "" {
		c.App.Env = "development"
	}
	if c.Admin.AuthCode == "" {
		c.Admin.AuthCode = "12345"
	}
	if c.Telegram.BaseURL == "" {
		c.Telegram.BaseURL = "https://api.telegram.org"
	}
	if c.Telegram.PollTimeoutSeconds <= 0 {
		c.Telegram.PollTimeoutSeconds = 30
	}
	if c.Fees.TransactionPercent == "" {
		c.Fees.TransactionPercent = "2"
	}
	if c.Market.Provider == "" {
		c.Market.Provider = "kucoin"
	}
	c.Market.Provider = strings.ToLower(strings.TrimSpace(c.Market.Provider))
	if c.Market.QuoteCurrency == "" {
		c.Market.QuoteCurrency = "USDT"
	}
	if c.Market.SettlementCurrency == "" {
		c.Market.SettlementCurrency = "TMN"
	}
	c.Market.QuoteCurrency = strings.ToUpper(strings.TrimSpace(c.Market.QuoteCurrency))
	c.Market.SettlementCurrency = strings.ToUpper(strings.TrimSpace(c.Market.SettlementCurrency))
	if c.Market.USDTToTMNRate != "" {
		c.Market.QuoteToSettlementRate = c.Market.USDTToTMNRate
	}
	if c.Market.QuoteToSettlementRate == "" {
		c.Market.QuoteToSettlementRate = "1"
	}
	if c.Providers.Kucoin.BaseURL == "" {
		c.Providers.Kucoin.BaseURL = "https://api.kucoin.com"
	}
	if c.Providers.Kucoin.TimeoutSeconds <= 0 {
		c.Providers.Kucoin.TimeoutSeconds = 10
	}
	for i := range c.Market.Coins {
		c.Market.Coins[i].Symbol = strings.ToUpper(strings.TrimSpace(c.Market.Coins[i].Symbol))
		if c.Market.Coins[i].Name == "" {
			c.Market.Coins[i].Name = c.Market.Coins[i].Symbol
		}
	}
}

func (c Config) validate() error {
	var errs []error

	if strings.TrimSpace(c.Telegram.BotToken) == "" || strings.Contains(c.Telegram.BotToken, "REPLACE_WITH") {
		errs = append(errs, errors.New("telegram.bot_token must be configured"))
	}
	if strings.TrimSpace(c.Database.Address) == "" {
		errs = append(errs, errors.New("database.address is required"))
	}
	if len(c.EnabledCoins()) == 0 {
		errs = append(errs, errors.New("market.coins must include at least one enabled coin"))
	}
	feePercent, err := decimal.NewFromString(c.Fees.TransactionPercent)
	if err != nil {
		errs = append(errs, fmt.Errorf("fees.transaction_percent: %w", err))
	} else if feePercent.LessThan(decimal.Zero) || feePercent.GreaterThan(decimal.NewFromInt(100)) {
		errs = append(errs, errors.New("fees.transaction_percent must be between 0 and 100"))
	}
	if _, err := decimal.NewFromString(c.Market.QuoteToSettlementRate); err != nil {
		errs = append(errs, fmt.Errorf("market.quote_to_settlement_rate: %w", err))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (c Config) EnabledCoins() []CoinConfig {
	coins := make([]CoinConfig, 0, len(c.Market.Coins))
	for _, coin := range c.Market.Coins {
		if coin.IsEnabled() {
			coins = append(coins, coin)
		}
	}
	return coins
}

func (c Config) CoinSymbols() []string {
	symbols := make([]string, 0, len(c.EnabledCoins())+1)
	for _, coin := range c.EnabledCoins() {
		symbols = append(symbols, coin.Symbol)
	}
	symbols = append(symbols, c.Market.SettlementCurrency)
	return symbols
}

func (c Config) QuoteToSettlementDecimal() decimal.Decimal {
	return decimal.RequireFromString(c.Market.QuoteToSettlementRate)
}

func (c Config) TransactionFeePercentDecimal() decimal.Decimal {
	return decimal.RequireFromString(c.Fees.TransactionPercent)
}

func (c Config) TransactionFeeRateDecimal() decimal.Decimal {
	return c.TransactionFeePercentDecimal().Div(decimal.NewFromInt(100))
}
