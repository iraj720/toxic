# Telegram Crypto Exchange Bot

An enterprise-grade Telegram bot for buying, selling, managing, and transferring
crypto with a smooth, trustworthy, and highly guided user experience.

This product is designed for users who want the simplicity of chat and the
clarity of a modern fintech app. Inside Telegram, users can start chatting with
the bot and immediately access their account, view available coins and live
prices, buy or sell assets, maintain a TMN balance, add trusted contacts,
transfer crypto to other users, and share their contact card so others can send
funds to them quickly.

The project should be implemented in Go with a configuration-driven,
service-oriented structure so infrastructure and third-party integrations can be
replaced without rewriting business logic.

## Current Implementation

This repository now contains a working Go implementation with:

- Telegram long-polling bot runtime
- Implicit account creation on first message
- PostgreSQL-backed users, balances, contacts, and transactions
- Guided coin buy/sell, receipt-based deposit, contact, and transfer flows
- KuCoin-backed live pricing behind a swappable market interface with a 20-second cache
- Dynamic quote-to-settlement rate override managed by admins with config-backed default fallback
- Config-driven transaction fee support for buys, sells, and internal transfers with a 2% default
- Fully separated localization layer with Persian default and English support
- YAML-based runtime configuration
- Docker Compose for local PostgreSQL

## Quick Start

### 1. Create a Telegram bot token

- Open BotFather in Telegram
- Create a new bot
- Copy the generated bot token

### 2. Update `config.yaml`

Set at least:

- `telegram.bot_token`
- `database.address`
- `market.coins`
- `market.quote_to_settlement_rate` or `market.usdt_to_tmn_rate`
- `fees.transaction_percent`
- `providers.kucoin.base_url`

### 3. Start PostgreSQL

```bash
docker-compose up -d postgres
```

### 4. Run the bot

```bash
go run ./cmd/exchangebot -config config.yaml
```

To keep the bot running after you disconnect from a server, install it as a
`systemd` service with:

```bash
./process.sh install
./process.sh start
```

### 5. Start using it in Telegram

- Open your bot chat
- Send any first message
- Your account and wallet will be created automatically
- Use the menu buttons to open your profile, browse coins, transfer assets,
  share your contact, submit a deposit receipt, and review transaction history

## Local Verification

The project has been verified with:

- `go build ./cmd/exchangebot`
- `go test ./...`
- `go test ./internal/store -run TestPostgresStoreSmoke -count=1`

The store smoke test validates migrations plus a basic user creation and TMN
deposit flow against a real PostgreSQL instance.

## Project Layout

- `cmd/exchangebot`: application entrypoint
- `internal/app`: app bootstrap and dependency wiring
- `internal/config`: YAML config loading and validation
- `internal/domain`: core product models
- `internal/service`: business logic and session state
- `internal/store`: PostgreSQL persistence and migration logic
- `internal/telegram`: Telegram Bot API client and chat workflows
- `pkg/market`: pricing provider interface
- `pkg/market/kucoin`: KuCoin implementation

## Product Vision

Build a Telegram-first exchange experience that feels as polished and reliable
as an enterprise financial platform while staying fast, simple, and accessible.

## Core Features

- Implicit account creation on first message to the bot
- Secure user profile and wallet setup
- Market view with available coins and current prices
- Buy crypto using TMN balance
- Sell crypto back to TMN
- TMN deposit flow for increasing available balance
- Config-driven transaction fees for coin buys, sells, and internal transfers
- TMN withdrawal request flow with temporary service-unavailable handling after confirmation
- Portfolio dashboard with balances and estimated values
- Internal crypto transfers between platform users
- Contact management for repeat transfers
- Contact sharing so users can be discovered and paid more easily
- Transaction history, receipts, and confirmation screens
- Guided error states and safe recovery flows

## Supported User Actions

### 1. Start using the bot instantly

Users do not need a separate registration flow.

When a user sends their first message to the bot:

- The system automatically creates their account
- A wallet is provisioned implicitly
- The user is taken directly to the main dashboard
- The bot can show a short welcome and safety introduction

### 2. Explore markets

Users can open a market screen and see:

- Available coins
- Current price of each coin
- Price change indicators
- Buy and sell entry points
- Coin detail pages with balance, charts, and actions

### 3. Buy crypto

Users can:

- Select a coin
- Enter an amount in TMN or coin quantity
- Review fees, rate, and final amount
- Confirm the trade
- Receive a clear success receipt

### 4. Sell crypto

Users can:

- Choose a coin from their wallet
- Enter the amount to sell
- Review rate, fees, and TMN received
- Confirm the transaction
- See updated balances immediately

### 5. Increase TMN balance

TMN acts as the base balance users can deposit and use for buying crypto.

Users can:

- Open the TMN wallet
- Choose `Deposit`
- Receive deposit instructions or payment details
- Complete the deposit
- See TMN balance updated after confirmation

### 6. Transfer crypto to others

Users can transfer supported crypto assets internally to other platform users.

The flow should support:

- Selecting a coin
- Choosing a recipient from saved contacts
- Searching by shared contact/profile
- Entering transfer amount
- Reviewing recipient, amount, and fee
- Confirming with a final approval step

### 7. Add and manage contacts

Users can build a trusted contact list for faster transfers.

Each contact can include:

- Display name
- Telegram handle or internal account reference
- Shared profile/contact card
- Optional notes or nickname

### 8. Share personal contact card

Users should be able to share their contact/profile from inside the bot so other
users can add them and send assets later.

This shareable contact experience should make receiving funds feel simple,
professional, and safe.

## UX and UI Principles

This bot should not feel like a basic command bot. It should feel like a modern
enterprise financial product delivered inside Telegram.

### Experience goals

- Clean, guided flows with minimal cognitive load
- Consistent layout and action hierarchy
- Fast access to balances, markets, and recent activity
- Clear confirmations before every sensitive action
- Strong trust signals in every screen
- Frictionless repeat actions for power users

### Enterprise-style bot experience

The bot UX should include:

- Structured menus instead of command-heavy interaction
- Inline keyboards for primary actions
- Card-like summaries for balances, quotes, and receipts
- Short, readable financial language
- Strong visual grouping of data
- Transaction previews before confirmation
- Success, pending, and failed states that are unmistakable
- Persistent navigation to `Home`, `Markets`, `Portfolio`, `Transfer`, and
  `Settings`

### Recommended navigation

- `Home`: portfolio summary, TMN balance, quick actions
- `Markets`: coin list, price snapshots, buy/sell
- `Wallet`: balances, deposit, withdraw, transaction history
- `Transfer`: contacts, send flow, shared profile lookup
- `Profile`: personal contact card, account settings, security

### UX quality standards

- Every trade or transfer must show a review step before execution
- Every successful action must return a receipt with transaction details
- Errors must explain what happened and what the user can do next
- High-risk actions must require explicit confirmation
- Repeated actions should become faster through saved contacts and recent assets

## Example User Journey

1. A user starts the bot and sends their first message.
2. The bot opens a clean dashboard showing TMN balance, crypto balance, and
   quick actions.
3. The user deposits TMN to fund the account.
4. The user opens `Markets`, selects BTC, and buys using TMN.
5. The user adds a friend as a contact.
6. The user later transfers USDT or another supported coin to that contact.
7. The user shares their own contact card with another person, who adds them and
   sends funds back.

## Functional Requirements

- Users must be automatically identified by their Telegram identity
- Users must receive a unique internal wallet/account on first interaction with
  the bot
- The system must support a configurable list of coins
- The system must display current prices for all enabled coins
- The system must support buy and sell operations against TMN
- The system must support TMN deposits
- The system must support TMN withdrawal requests, even while execution is temporarily unavailable
- The system must support internal transfers between users
- The system must support contact creation, storage, lookup, and deletion
- The system must support sharing a user contact/profile with others
- The system must maintain transaction history and status tracking
- The system must provide clear confirmations and receipts for every operation

## Non-Functional Requirements

- Modular architecture so pricing, wallet, payment, and messaging providers can
  be swapped later
- Go codebase with clear packages, interfaces, and testable services
- Secure handling of account, wallet, and transaction data
- Audit-friendly transaction records
- High clarity in messaging for financial trust
- Fast response time for common actions
- Extensible design for future web, mobile, or admin interfaces

## Technology Direction

- Language: Go
- Database: PostgreSQL
- Runtime configuration: `config.yaml`
- Bot platform: Telegram Bot API
- Live market pricing: KuCoin

## Configuration

The application should load runtime settings from `config.yaml`.

At minimum, configuration should include:

- Telegram bot token
- PostgreSQL connection address
- Enabled coins
- KuCoin integration settings
- Environment-specific operational settings

### Example `config.yaml`

```yaml
app:
  name: exchange-bot
  env: development

admin:
  auth_code: "12345"

fees:
  transaction_percent: "2"

telegram:
  bot_token: "YOUR_TELEGRAM_BOT_TOKEN"
  base_url: "https://api.telegram.org"
  poll_timeout_seconds: 30

database:
  address: "postgres://postgres:postgres@localhost:5433/exchange?sslmode=disable"

deposit:
  card_number: "6037-9918-0000-0000"

market:
  provider: kucoin
  quote_currency: USDT
  settlement_currency: TMN
  usdt_to_tmn_rate: "1"
  coins:
    - symbol: BTC
      name: Bitcoin
    - symbol: ETH
      name: Ethereum
    - symbol: USDT
      name: Tether
    - symbol: TON
      name: Toncoin

providers:
  kucoin:
    base_url: "https://api.kucoin.com"
    timeout_seconds: 10
```

This keeps deployment simple and makes it easy to change coins, infrastructure,
or provider configuration without changing the core application logic.
The configured quote-to-settlement rate acts as the default startup value and
can later be adjusted by an authenticated admin from inside the bot.

## Suggested System Modules

- `auth`: Telegram identity recognition and first-message account bootstrap
- `accounts`: user profile, wallet account, status
- `assets`: supported coins and metadata
- `pricing`: price feeds and quote generation
- `trading`: buy and sell execution
- `tmn-wallet`: TMN balance, deposit, reconciliation
- `transfers`: internal crypto transfer workflows
- `contacts`: saved contacts and shared profile handling
- `ledger`: transaction history and audit trail
- `notifications`: receipts, alerts, status updates
- `config`: load and validate `config.yaml`
- `storage`: PostgreSQL repositories and persistence layer
- `integrations`: third-party service clients such as KuCoin

## Third-Party Pricing Integration

KuCoin should be integrated as the initial live price provider for market data.

To keep the system maintainable and extendable, third-party pricing integrations
should be abstracted behind interfaces in a reusable `pkg` package so other
providers can be added later without changing trading or bot flows.

### Recommended package direction

- `pkg/market`: shared provider interfaces and domain models
- `pkg/market/kucoin`: KuCoin implementation of the pricing interface
- `internal/service`: application service layer that depends on the interface,
  not the concrete provider

### Example interface direction

```go
package market

import "context"

type Quote struct {
	BaseSymbol  string
	QuoteSymbol string
	Price       string
	Source      string
}

type PriceProvider interface {
	GetPrice(ctx context.Context, baseSymbol, quoteSymbol string) (Quote, error)
}
```

With this approach, KuCoin becomes only one implementation. New providers can
be added later by implementing the same interface.

## Bot Experience

The current bot flow is menu-driven and optimized for clarity:

- `Profile`: shows the user summary, TMN total, coin balances, contacts, and language settings
- `Coins`: shows live prices, then lets the user tap a coin and choose buy or sell
- `Transfer`: asks for recipient, coin, amount, and confirmation
- `Share Contact`: shows the user's own share code
- `Deposit`: shows the configured card number, then waits for a receipt photo and records the deposit as pending
- `Withdraw TMN`: collects the withdrawal amount, asks for confirmation, and currently informs the user that withdrawals are temporarily unavailable
- `Transaction History`: shows transfers, deposits, buys, and sells with their statuses
- The app defaults to Persian and can be switched to English from `Profile -> Language`
- Admins can authenticate with `/auth 12345`, open `Admin Panel`, review pending deposits, approve them to credit TMN and mark them as `success`, and update the active quote-to-settlement rate used for pricing

## Trust, Security, and Compliance Expectations

- Confirm all sensitive actions before execution
- Prevent transfers to invalid or blocked recipients
- Record every trade, deposit, and transfer with immutable references
- Protect balances from duplicate execution and race conditions
- Clearly separate pending, completed, and failed transactions
- Add fraud checks, limits, and operational controls where required

## Future Expansion

- External wallet withdrawals and deposits
- KYC and compliance workflows
- Admin dashboard for support and reconciliation
- Multi-language support
- Advanced trading features
- Referral and rewards programs
- Web and mobile companion apps

## Assumptions

- `TMN` is the base balance or fiat-like settlement currency used inside the
  platform for deposits, buys, and sells.
- Users transfer crypto internally to other users on the same platform unless an
  external withdrawal feature is added later.
- "Share contact" means sharing an internal profile/contact card that another
  bot user can save and use as a transfer recipient.
- Market prices are initially provided by KuCoin and shown in the bot as current
  reference prices, with a 20-second in-memory cache to avoid unnecessary API calls.
- The product aims for an enterprise-grade user experience, meaning the bot
  should prioritize clarity, reliability, guided flows, and strong trust signals
  over minimal or purely command-based interaction.
- Configuration is stored in `config.yaml`, including Telegram credentials,
  PostgreSQL address, enabled coins, and provider settings.
- The Go codebase should use interfaces around third-party integrations so
  services such as KuCoin can be replaced later with minimal disruption.
- TMN to USDT pricing is configured in `config.yaml` as the default fallback so
  the bot can convert KuCoin USDT prices into TMN values, while admins can
  override the active runtime rate without editing the config file.
- Transaction fees for buys, sells, and internal transfers are configured in
  `config.yaml`, default to 2%, and are applied in TMN for buys and sells and
  in the transferred coin for internal transfers.
- Localization is fully separated from business logic in its own package, with
  Persian as the default locale and English as an alternative.
- TMN labels shown to users in bot messages are annotated with a human-friendly
  toman hint, and sensitive confirmation steps also render the entered amount
  as a toman phrase to reduce amount-entry mistakes, for example `TMN (هزار
  تومن)` and `1 TMN` as `1 thousand toman` / `۱ هزار تومان`.
- TMN deposit is currently implemented as a receipt-based pending workflow: the
  user enters an amount, sees the configured card number, uploads a receipt
  photo, and the transaction is stored as pending until it is reviewed.
- TMN withdrawal is currently exposed as a guided request flow only: the user
  enters an amount, confirms the request, and is then informed that
  withdrawals are temporarily unavailable because of current conditions.
- Admin authentication is currently handled through the Telegram command
  `/auth <code>` and remains active only for the current bot process lifetime.
