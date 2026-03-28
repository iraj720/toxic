package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	TransactionDeposit     = "deposit"
	TransactionBuy         = "buy"
	TransactionSell        = "sell"
	TransactionTransferIn  = "transfer_in"
	TransactionTransferOut = "transfer_out"
)

type TelegramProfile struct {
	TelegramUserID int64
	ChatID         int64
	Username       string
	FirstName      string
	LastName       string
}

type User struct {
	ID             int64
	TelegramUserID int64
	ChatID         int64
	Username       string
	FirstName      string
	LastName       string
	ShareCode      string
	Locale         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (u User) DisplayName() string {
	if strings.TrimSpace(u.FirstName+" "+u.LastName) != "" {
		return strings.TrimSpace(u.FirstName + " " + u.LastName)
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf("User %d", u.TelegramUserID)
}

type Balance struct {
	Asset     string
	Available decimal.Decimal
}

type Contact struct {
	ContactUserID int64
	Alias         string
	DisplayName   string
	Username      string
	ShareCode     string
	CreatedAt     time.Time
}

type Transaction struct {
	ID               string
	Kind             string
	UserID           int64
	CounterpartyID   *int64
	AssetCode        string
	AssetAmount      decimal.Decimal
	SettlementAsset  string
	SettlementAmount decimal.Decimal
	Price            decimal.Decimal
	Reference        string
	Status           string
	Description      string
	CreatedAt        time.Time
}

type Dashboard struct {
	User         User
	Balances     []Balance
	Contacts     []Contact
	Transactions []Transaction
}

type PendingDeposit struct {
	Transaction     Transaction
	UserDisplayName string
	ShareCode       string
}

type MarketPrice struct {
	Asset              string
	QuoteCurrency      string
	SettlementCurrency string
	PriceInQuote       decimal.Decimal
	PriceInSettlement  decimal.Decimal
	Source             string
	FetchedAt          time.Time
}

type QuoteToSettlementRate struct {
	QuoteCurrency      string
	SettlementCurrency string
	Rate               decimal.Decimal
	Source             string
}

type TradeQuote struct {
	Asset             string
	AssetAmount       decimal.Decimal
	SettlementAsset   string
	SettlementAmount  decimal.Decimal
	PriceInSettlement decimal.Decimal
	PriceInQuote      decimal.Decimal
	QuoteCurrency     string
	Source            string
}
