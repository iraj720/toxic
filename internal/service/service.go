package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"exchange/internal/config"
	"exchange/internal/domain"
	"exchange/pkg/market"
)

var (
	ErrUnknownAsset      = errors.New("unknown asset")
	ErrInvalidAmount     = errors.New("amount must be greater than zero")
	ErrInvalidRate       = errors.New("quote to settlement rate must be greater than zero")
	ErrContactNotFound   = errors.New("contact not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

type Store interface {
	EnsureUser(ctx context.Context, profile domain.TelegramProfile) (domain.User, bool, error)
	GetUserByTelegramID(ctx context.Context, telegramUserID int64) (domain.User, error)
	SetUserLocale(ctx context.Context, userID int64, locale string) error
	GetDashboard(ctx context.Context, userID int64, assets []string) (domain.Dashboard, error)
	AddContactByShareCode(ctx context.Context, ownerUserID int64, shareCode string) (domain.Contact, error)
	ResolveRecipient(ctx context.Context, ownerUserID int64, reference string) (domain.User, error)
	CreatePendingDeposit(ctx context.Context, userID int64, asset string, amount decimal.Decimal, receiptFileID, description string) (domain.Transaction, error)
	ListPendingDeposits(ctx context.Context, limit int) ([]domain.PendingDeposit, error)
	ApprovePendingDeposit(ctx context.Context, transactionID, settlementAsset string) (domain.Transaction, error)
	Deposit(ctx context.Context, userID int64, asset string, amount decimal.Decimal) (domain.Transaction, error)
	Buy(ctx context.Context, userID int64, quote domain.TradeQuote) (domain.Transaction, error)
	Sell(ctx context.Context, userID int64, quote domain.TradeQuote) (domain.Transaction, error)
	Transfer(ctx context.Context, senderUserID, recipientUserID int64, quote domain.TransferQuote) (domain.Transaction, error)
	GetQuoteToSettlementRate(ctx context.Context) (decimal.Decimal, bool, error)
	SetQuoteToSettlementRate(ctx context.Context, rate decimal.Decimal) error
}

type Service struct {
	cfg      config.Config
	store    Store
	provider market.PriceProvider
	sessions *SessionManager
}

func New(cfg config.Config, store Store, provider market.PriceProvider) *Service {
	return &Service{
		cfg:      cfg,
		store:    store,
		provider: provider,
		sessions: NewSessionManager(),
	}
}

func (s *Service) Sessions() *SessionManager {
	return s.sessions
}

func (s *Service) EnsureUser(ctx context.Context, profile domain.TelegramProfile) (domain.User, bool, error) {
	return s.store.EnsureUser(ctx, profile)
}

func (s *Service) Dashboard(ctx context.Context, telegramUserID int64) (domain.Dashboard, error) {
	user, err := s.store.GetUserByTelegramID(ctx, telegramUserID)
	if err != nil {
		return domain.Dashboard{}, err
	}
	return s.store.GetDashboard(ctx, user.ID, s.cfg.CoinSymbols())
}

func (s *Service) SettlementCurrency() string {
	return s.cfg.Market.SettlementCurrency
}

func (s *Service) DepositCardNumber() string {
	return strings.TrimSpace(s.cfg.Deposit.CardNumber)
}

func (s *Service) AuthenticateAdmin(code string) bool {
	return strings.TrimSpace(code) != "" && strings.TrimSpace(code) == strings.TrimSpace(s.cfg.Admin.AuthCode)
}

func (s *Service) SetLocale(ctx context.Context, telegramUserID int64, locale string) error {
	user, err := s.store.GetUserByTelegramID(ctx, telegramUserID)
	if err != nil {
		return err
	}
	return s.store.SetUserLocale(ctx, user.ID, locale)
}

func (s *Service) EnabledCoinSymbols() []string {
	coins := s.cfg.EnabledCoins()
	out := make([]string, 0, len(coins))
	for _, coin := range coins {
		out = append(out, coin.Symbol)
	}
	return out
}

func (s *Service) TransactionFeePercent() decimal.Decimal {
	return s.cfg.TransactionFeePercentDecimal()
}

func (s *Service) CurrentQuoteToSettlementRate(ctx context.Context) (domain.QuoteToSettlementRate, error) {
	rate := domain.QuoteToSettlementRate{
		QuoteCurrency:      s.cfg.Market.QuoteCurrency,
		SettlementCurrency: s.cfg.Market.SettlementCurrency,
		Rate:               s.cfg.QuoteToSettlementDecimal(),
		Source:             "config",
	}

	override, ok, err := s.store.GetQuoteToSettlementRate(ctx)
	if err != nil {
		return domain.QuoteToSettlementRate{}, err
	}
	if ok {
		rate.Rate = override
		rate.Source = "admin"
	}

	return rate, nil
}

func (s *Service) SetQuoteToSettlementRate(ctx context.Context, rate decimal.Decimal) (domain.QuoteToSettlementRate, error) {
	if rate.LessThanOrEqual(decimal.Zero) {
		return domain.QuoteToSettlementRate{}, ErrInvalidRate
	}
	if err := s.store.SetQuoteToSettlementRate(ctx, rate); err != nil {
		return domain.QuoteToSettlementRate{}, err
	}
	return s.CurrentQuoteToSettlementRate(ctx)
}

func (s *Service) ListMarketPrices(ctx context.Context) ([]domain.MarketPrice, error) {
	coins := s.cfg.EnabledCoins()
	rate, err := s.CurrentQuoteToSettlementRate(ctx)
	if err != nil {
		return nil, err
	}

	prices := make([]domain.MarketPrice, 0, len(coins))
	for _, coin := range coins {
		priceInQuote := decimal.NewFromInt(1)
		source := rate.Source
		fetchedAt := time.Now().UTC()
		if coin.Symbol != s.cfg.Market.QuoteCurrency {
			quote, err := s.provider.GetPrice(ctx, coin.Symbol, s.cfg.Market.QuoteCurrency)
			if err != nil {
				return nil, err
			}
			priceInQuote = quote.Price
			source = composePriceSource(quote.Source, rate.Source)
			prices = append(prices, domain.MarketPrice{
				Asset:              coin.Symbol,
				QuoteCurrency:      s.cfg.Market.QuoteCurrency,
				SettlementCurrency: s.cfg.Market.SettlementCurrency,
				PriceInQuote:       quote.Price,
				PriceInSettlement:  quote.Price.Mul(rate.Rate),
				Source:             source,
				FetchedAt:          quote.FetchedAt,
			})
			continue
		}
		prices = append(prices, domain.MarketPrice{
			Asset:              coin.Symbol,
			QuoteCurrency:      s.cfg.Market.QuoteCurrency,
			SettlementCurrency: s.cfg.Market.SettlementCurrency,
			PriceInQuote:       priceInQuote,
			PriceInSettlement:  priceInQuote.Mul(rate.Rate),
			Source:             source,
			FetchedAt:          fetchedAt,
		})
	}

	sort.Slice(prices, func(i, j int) bool {
		return prices[i].Asset < prices[j].Asset
	})

	return prices, nil
}

func (s *Service) CoinPrice(ctx context.Context, asset string) (domain.MarketPrice, error) {
	return s.getSettlementPrice(ctx, asset)
}

func (s *Service) QuoteBuy(ctx context.Context, asset string, settlementAmount decimal.Decimal) (domain.TradeQuote, error) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if settlementAmount.LessThanOrEqual(decimal.Zero) {
		return domain.TradeQuote{}, ErrInvalidAmount
	}
	if !s.isEnabledAsset(asset) {
		return domain.TradeQuote{}, ErrUnknownAsset
	}

	price, err := s.getSettlementPrice(ctx, asset)
	if err != nil {
		return domain.TradeQuote{}, err
	}
	if price.PriceInSettlement.LessThanOrEqual(decimal.Zero) {
		return domain.TradeQuote{}, fmt.Errorf("price is not positive for %s", asset)
	}

	feeAmount := settlementAmount.Mul(s.cfg.TransactionFeeRateDecimal())
	totalSettlementAmount := settlementAmount.Add(feeAmount)

	return domain.TradeQuote{
		Asset:                 asset,
		AssetAmount:           settlementAmount.Div(price.PriceInSettlement),
		SettlementAsset:       s.cfg.Market.SettlementCurrency,
		GrossSettlementAmount: settlementAmount,
		SettlementAmount:      totalSettlementAmount,
		FeeAsset:              s.cfg.Market.SettlementCurrency,
		FeeAmount:             feeAmount,
		PriceInSettlement:     price.PriceInSettlement,
		PriceInQuote:          price.PriceInQuote,
		QuoteCurrency:         price.QuoteCurrency,
		Source:                price.Source,
	}, nil
}

func (s *Service) QuoteSell(ctx context.Context, asset string, assetAmount decimal.Decimal) (domain.TradeQuote, error) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if assetAmount.LessThanOrEqual(decimal.Zero) {
		return domain.TradeQuote{}, ErrInvalidAmount
	}
	if !s.isEnabledAsset(asset) {
		return domain.TradeQuote{}, ErrUnknownAsset
	}

	price, err := s.getSettlementPrice(ctx, asset)
	if err != nil {
		return domain.TradeQuote{}, err
	}

	grossSettlementAmount := assetAmount.Mul(price.PriceInSettlement)
	feeAmount := grossSettlementAmount.Mul(s.cfg.TransactionFeeRateDecimal())

	return domain.TradeQuote{
		Asset:                 asset,
		AssetAmount:           assetAmount,
		SettlementAsset:       s.cfg.Market.SettlementCurrency,
		GrossSettlementAmount: grossSettlementAmount,
		SettlementAmount:      grossSettlementAmount.Sub(feeAmount),
		FeeAsset:              s.cfg.Market.SettlementCurrency,
		FeeAmount:             feeAmount,
		PriceInSettlement:     price.PriceInSettlement,
		PriceInQuote:          price.PriceInQuote,
		QuoteCurrency:         price.QuoteCurrency,
		Source:                price.Source,
	}, nil
}

func (s *Service) QuoteTransfer(asset string, amount decimal.Decimal) (domain.TransferQuote, error) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if amount.LessThanOrEqual(decimal.Zero) {
		return domain.TransferQuote{}, ErrInvalidAmount
	}
	if !s.isEnabledAsset(asset) {
		return domain.TransferQuote{}, ErrUnknownAsset
	}

	feeAmount := amount.Mul(s.cfg.TransactionFeeRateDecimal())

	return domain.TransferQuote{
		Asset:            asset,
		AssetAmount:      amount,
		FeeAsset:         asset,
		FeeAmount:        feeAmount,
		TotalDebitAmount: amount.Add(feeAmount),
	}, nil
}

func (s *Service) CreatePendingDeposit(ctx context.Context, telegramUserID int64, amount decimal.Decimal, receiptFileID string) (domain.Transaction, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return domain.Transaction{}, ErrInvalidAmount
	}
	user, err := s.store.GetUserByTelegramID(ctx, telegramUserID)
	if err != nil {
		return domain.Transaction{}, err
	}
	return s.store.CreatePendingDeposit(
		ctx,
		user.ID,
		s.cfg.Market.SettlementCurrency,
		amount,
		receiptFileID,
		fmt.Sprintf("Manual deposit receipt submitted for card %s", s.DepositCardNumber()),
	)
}

func (s *Service) ListPendingDeposits(ctx context.Context, limit int) ([]domain.PendingDeposit, error) {
	return s.store.ListPendingDeposits(ctx, limit)
}

func (s *Service) ApprovePendingDeposit(ctx context.Context, transactionID string) (domain.Transaction, error) {
	return s.store.ApprovePendingDeposit(ctx, transactionID, s.cfg.Market.SettlementCurrency)
}

func (s *Service) Deposit(ctx context.Context, telegramUserID int64, amount decimal.Decimal) (domain.Transaction, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return domain.Transaction{}, ErrInvalidAmount
	}
	user, err := s.store.GetUserByTelegramID(ctx, telegramUserID)
	if err != nil {
		return domain.Transaction{}, err
	}
	return s.store.Deposit(ctx, user.ID, s.cfg.Market.SettlementCurrency, amount)
}

func (s *Service) Buy(ctx context.Context, telegramUserID int64, quote domain.TradeQuote) (domain.Transaction, error) {
	user, err := s.store.GetUserByTelegramID(ctx, telegramUserID)
	if err != nil {
		return domain.Transaction{}, err
	}
	return s.store.Buy(ctx, user.ID, quote)
}

func (s *Service) Sell(ctx context.Context, telegramUserID int64, quote domain.TradeQuote) (domain.Transaction, error) {
	user, err := s.store.GetUserByTelegramID(ctx, telegramUserID)
	if err != nil {
		return domain.Transaction{}, err
	}
	return s.store.Sell(ctx, user.ID, quote)
}

func (s *Service) AddContact(ctx context.Context, telegramUserID int64, shareCode string) (domain.Contact, error) {
	user, err := s.store.GetUserByTelegramID(ctx, telegramUserID)
	if err != nil {
		return domain.Contact{}, err
	}
	return s.store.AddContactByShareCode(ctx, user.ID, shareCode)
}

func (s *Service) ResolveRecipient(ctx context.Context, telegramUserID int64, reference string) (domain.User, error) {
	user, err := s.store.GetUserByTelegramID(ctx, telegramUserID)
	if err != nil {
		return domain.User{}, err
	}
	return s.store.ResolveRecipient(ctx, user.ID, reference)
}

func (s *Service) Transfer(ctx context.Context, telegramUserID int64, recipientUserID int64, quote domain.TransferQuote) (domain.Transaction, error) {
	if quote.AssetAmount.LessThanOrEqual(decimal.Zero) {
		return domain.Transaction{}, ErrInvalidAmount
	}
	if !s.isEnabledAsset(quote.Asset) && strings.ToUpper(quote.Asset) != s.cfg.Market.SettlementCurrency {
		return domain.Transaction{}, ErrUnknownAsset
	}
	user, err := s.store.GetUserByTelegramID(ctx, telegramUserID)
	if err != nil {
		return domain.Transaction{}, err
	}
	return s.store.Transfer(ctx, user.ID, recipientUserID, quote)
}

func (s *Service) getSettlementPrice(ctx context.Context, asset string) (domain.MarketPrice, error) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	rate, err := s.CurrentQuoteToSettlementRate(ctx)
	if err != nil {
		return domain.MarketPrice{}, err
	}

	if asset == s.cfg.Market.QuoteCurrency {
		return domain.MarketPrice{
			Asset:              asset,
			QuoteCurrency:      s.cfg.Market.QuoteCurrency,
			SettlementCurrency: s.cfg.Market.SettlementCurrency,
			PriceInQuote:       decimal.NewFromInt(1),
			PriceInSettlement:  rate.Rate,
			Source:             rate.Source,
		}, nil
	}

	quote, err := s.provider.GetPrice(ctx, asset, s.cfg.Market.QuoteCurrency)
	if err != nil {
		return domain.MarketPrice{}, err
	}
	return domain.MarketPrice{
		Asset:              asset,
		QuoteCurrency:      s.cfg.Market.QuoteCurrency,
		SettlementCurrency: s.cfg.Market.SettlementCurrency,
		PriceInQuote:       quote.Price,
		PriceInSettlement:  quote.Price.Mul(rate.Rate),
		Source:             composePriceSource(quote.Source, rate.Source),
		FetchedAt:          quote.FetchedAt,
	}, nil
}

func (s *Service) isEnabledAsset(asset string) bool {
	for _, symbol := range s.EnabledCoinSymbols() {
		if symbol == strings.ToUpper(asset) {
			return true
		}
	}
	return false
}

func composePriceSource(priceSource, rateSource string) string {
	if strings.TrimSpace(rateSource) == "" || rateSource == "config" {
		return priceSource
	}
	if strings.TrimSpace(priceSource) == "" {
		return "admin-rate"
	}
	return priceSource + " + admin-rate"
}

type Session struct {
	Step string
	Data map[string]string
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[int64]Session
	admins   map[int64]bool
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: map[int64]Session{},
		admins:   map[int64]bool{},
	}
}

func (m *SessionManager) Get(userID int64) (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[userID]
	return session, ok
}

func (m *SessionManager) Set(userID int64, session Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[userID] = session
}

func (m *SessionManager) Clear(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, userID)
}

func (m *SessionManager) AuthorizeAdmin(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.admins[userID] = true
}

func (m *SessionManager) RevokeAdmin(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.admins, userID)
}

func (m *SessionManager) IsAdmin(userID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.admins[userID]
}
