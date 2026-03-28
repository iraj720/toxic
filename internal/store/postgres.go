package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"

	"exchange/internal/domain"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidStatus     = errors.New("invalid transaction status")
)

const quoteToSettlementRateSettingKey = "market.quote_to_settlement_rate"

type PostgresStore struct {
	pool             *sql.DB
	settlementAsset  string
	configuredAssets []string
}

func New(ctx context.Context, address, settlementAsset string, configuredAssets []string) (*PostgresStore, error) {
	pool, err := sql.Open("postgres", address)
	if err != nil {
		return nil, err
	}
	if err := pool.PingContext(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &PostgresStore{
		pool:             pool,
		settlementAsset:  settlementAsset,
		configuredAssets: configuredAssets,
	}, nil
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}

func (s *PostgresStore) Migrate(ctx context.Context) error {
	_, err := s.pool.ExecContext(ctx, schemaSQL)
	return err
}

func (s *PostgresStore) EnsureUser(ctx context.Context, profile domain.TelegramProfile) (domain.User, bool, error) {
	var created bool
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, false, err
	}
	defer tx.Rollback()

	var user domain.User
	err = tx.QueryRowContext(ctx, `
		SELECT id, telegram_user_id, chat_id, username, first_name, last_name, share_code, locale, created_at, updated_at
		FROM users
		WHERE telegram_user_id = $1
	`, profile.TelegramUserID).Scan(
		&user.ID,
		&user.TelegramUserID,
		&user.ChatID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.ShareCode,
		&user.Locale,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err == nil {
		_, err = tx.ExecContext(ctx, `
			UPDATE users
			SET chat_id = $2, username = $3, first_name = $4, last_name = $5, updated_at = NOW()
			WHERE telegram_user_id = $1
		`, profile.TelegramUserID, profile.ChatID, profile.Username, profile.FirstName, profile.LastName)
		if err != nil {
			return domain.User{}, false, err
		}
		user.ChatID = profile.ChatID
		user.Username = profile.Username
		user.FirstName = profile.FirstName
		user.LastName = profile.LastName
		user.UpdatedAt = time.Now().UTC()
	} else {
		if !errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, false, err
		}

		created = true
		for {
			shareCode := newShareCode()
			err = tx.QueryRowContext(ctx, `
				INSERT INTO users (telegram_user_id, chat_id, username, first_name, last_name, share_code)
				VALUES ($1, $2, $3, $4, $5, $6)
				RETURNING id, telegram_user_id, chat_id, username, first_name, last_name, share_code, locale, created_at, updated_at
			`, profile.TelegramUserID, profile.ChatID, profile.Username, profile.FirstName, profile.LastName, shareCode).Scan(
				&user.ID,
				&user.TelegramUserID,
				&user.ChatID,
				&user.Username,
				&user.FirstName,
				&user.LastName,
				&user.ShareCode,
				&user.Locale,
				&user.CreatedAt,
				&user.UpdatedAt,
			)
			if err == nil {
				break
			}
			if !strings.Contains(strings.ToLower(err.Error()), "share_code") {
				return domain.User{}, false, err
			}
		}
	}

	for _, asset := range s.configuredAssets {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO balances (user_id, asset_code, amount)
			VALUES ($1, $2, 0)
			ON CONFLICT (user_id, asset_code) DO NOTHING
		`, user.ID, asset); err != nil {
			return domain.User{}, false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.User{}, false, err
	}

	return user, created, nil
}

func (s *PostgresStore) GetUserByTelegramID(ctx context.Context, telegramUserID int64) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRowContext(ctx, `
		SELECT id, telegram_user_id, chat_id, username, first_name, last_name, share_code, locale, created_at, updated_at
		FROM users
		WHERE telegram_user_id = $1
	`, telegramUserID).Scan(
		&user.ID,
		&user.TelegramUserID,
		&user.ChatID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.ShareCode,
		&user.Locale,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) SetUserLocale(ctx context.Context, userID int64, locale string) error {
	_, err := s.pool.ExecContext(ctx, `
		UPDATE users
		SET locale = $2, updated_at = NOW()
		WHERE id = $1
	`, userID, locale)
	return err
}

func (s *PostgresStore) GetQuoteToSettlementRate(ctx context.Context) (decimal.Decimal, bool, error) {
	var raw string
	err := s.pool.QueryRowContext(ctx, `
		SELECT value
		FROM app_settings
		WHERE key = $1
	`, quoteToSettlementRateSettingKey).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return decimal.Zero, false, nil
		}
		return decimal.Zero, false, err
	}

	rate, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, false, err
	}
	return rate, true, nil
}

func (s *PostgresStore) SetQuoteToSettlementRate(ctx context.Context, rate decimal.Decimal) error {
	_, err := s.pool.ExecContext(ctx, `
		INSERT INTO app_settings (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, quoteToSettlementRateSettingKey, rate.String())
	return err
}

func (s *PostgresStore) GetDashboard(ctx context.Context, userID int64, assets []string) (domain.Dashboard, error) {
	var dashboard domain.Dashboard
	var err error
	dashboard.User, err = s.getUserByID(ctx, userID)
	if err != nil {
		return domain.Dashboard{}, err
	}
	dashboard.Balances, err = s.listBalances(ctx, userID, assets)
	if err != nil {
		return domain.Dashboard{}, err
	}
	dashboard.Contacts, err = s.listContacts(ctx, userID)
	if err != nil {
		return domain.Dashboard{}, err
	}
	dashboard.Transactions, err = s.listTransactions(ctx, userID, 10)
	if err != nil {
		return domain.Dashboard{}, err
	}
	return dashboard, nil
}

func (s *PostgresStore) AddContactByShareCode(ctx context.Context, ownerUserID int64, shareCode string) (domain.Contact, error) {
	shareCode = strings.ToUpper(strings.TrimSpace(shareCode))
	recipient, err := s.getUserByShareCode(ctx, shareCode)
	if err != nil {
		return domain.Contact{}, err
	}
	if recipient.ID == ownerUserID {
		return domain.Contact{}, fmt.Errorf("cannot add yourself as a contact")
	}

	alias := recipient.DisplayName()
	if recipient.Username != "" {
		alias = "@" + recipient.Username
	}

	_, err = s.pool.ExecContext(ctx, `
		INSERT INTO contacts (owner_user_id, contact_user_id, alias)
		VALUES ($1, $2, $3)
		ON CONFLICT (owner_user_id, contact_user_id)
		DO UPDATE SET alias = EXCLUDED.alias
	`, ownerUserID, recipient.ID, alias)
	if err != nil {
		return domain.Contact{}, err
	}

	return domain.Contact{
		ContactUserID: recipient.ID,
		Alias:         alias,
		DisplayName:   recipient.DisplayName(),
		Username:      recipient.Username,
		ShareCode:     recipient.ShareCode,
	}, nil
}

func (s *PostgresStore) ResolveRecipient(ctx context.Context, ownerUserID int64, reference string) (domain.User, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return domain.User{}, ErrNotFound
	}
	if user, err := s.getUserByShareCode(ctx, strings.ToUpper(reference)); err == nil {
		if user.ID == ownerUserID {
			return domain.User{}, fmt.Errorf("cannot transfer to yourself")
		}
		return user, nil
	}

	var user domain.User
	err := s.pool.QueryRowContext(ctx, `
		SELECT u.id, u.telegram_user_id, u.chat_id, u.username, u.first_name, u.last_name, u.share_code, u.locale, u.created_at, u.updated_at
		FROM contacts c
		JOIN users u ON u.id = c.contact_user_id
		WHERE c.owner_user_id = $1 AND LOWER(c.alias) = LOWER($2)
		LIMIT 1
	`, ownerUserID, reference).Scan(
		&user.ID,
		&user.TelegramUserID,
		&user.ChatID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.ShareCode,
		&user.Locale,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) Deposit(ctx context.Context, userID int64, asset string, amount decimal.Decimal) (domain.Transaction, error) {
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return domain.Transaction{}, err
	}
	defer tx.Rollback()

	if err := s.incrementBalance(ctx, tx, userID, asset, amount); err != nil {
		return domain.Transaction{}, err
	}
	transaction, err := s.insertTransaction(ctx, tx, domain.Transaction{
		Kind:             domain.TransactionDeposit,
		UserID:           userID,
		AssetCode:        asset,
		AssetAmount:      amount,
		SettlementAsset:  asset,
		SettlementAmount: amount,
		Status:           "completed",
		Description:      "TMN balance deposit",
	})
	if err != nil {
		return domain.Transaction{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Transaction{}, err
	}
	return transaction, nil
}

func (s *PostgresStore) CreatePendingDeposit(ctx context.Context, userID int64, asset string, amount decimal.Decimal, receiptFileID, description string) (domain.Transaction, error) {
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return domain.Transaction{}, err
	}
	defer tx.Rollback()

	transaction, err := s.insertTransaction(ctx, tx, domain.Transaction{
		Kind:             domain.TransactionDeposit,
		UserID:           userID,
		AssetCode:        asset,
		AssetAmount:      amount,
		SettlementAsset:  asset,
		SettlementAmount: amount,
		Reference:        receiptFileID,
		Status:           "pending",
		Description:      description,
	})
	if err != nil {
		return domain.Transaction{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Transaction{}, err
	}
	return transaction, nil
}

func (s *PostgresStore) ListPendingDeposits(ctx context.Context, limit int) ([]domain.PendingDeposit, error) {
	rows, err := s.pool.QueryContext(ctx, `
		SELECT
			t.id, t.kind, t.user_id, t.counterparty_user_id, t.asset_code, t.asset_amount,
			t.settlement_asset_code, t.settlement_amount, t.price, t.reference, t.status, t.description, t.created_at,
			u.first_name, u.last_name, u.username, u.share_code
		FROM transactions t
		JOIN users u ON u.id = t.user_id
		WHERE t.kind = $1 AND t.status = 'pending'
		ORDER BY t.created_at ASC
		LIMIT $2
	`, domain.TransactionDeposit, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pending []domain.PendingDeposit
	for rows.Next() {
		var item domain.PendingDeposit
		var counterparty sql.NullInt64
		var assetAmount string
		var settlementAmount string
		var price string
		var firstName string
		var lastName string
		var username string
		if err := rows.Scan(
			&item.Transaction.ID,
			&item.Transaction.Kind,
			&item.Transaction.UserID,
			&counterparty,
			&item.Transaction.AssetCode,
			&assetAmount,
			&item.Transaction.SettlementAsset,
			&settlementAmount,
			&price,
			&item.Transaction.Reference,
			&item.Transaction.Status,
			&item.Transaction.Description,
			&item.Transaction.CreatedAt,
			&firstName,
			&lastName,
			&username,
			&item.ShareCode,
		); err != nil {
			return nil, err
		}
		if counterparty.Valid {
			value := counterparty.Int64
			item.Transaction.CounterpartyID = &value
		}
		item.Transaction.AssetAmount, err = decimal.NewFromString(assetAmount)
		if err != nil {
			return nil, err
		}
		item.Transaction.SettlementAmount, err = decimal.NewFromString(settlementAmount)
		if err != nil {
			return nil, err
		}
		item.Transaction.Price, err = decimal.NewFromString(price)
		if err != nil {
			return nil, err
		}
		item.UserDisplayName = strings.TrimSpace(firstName + " " + lastName)
		if item.UserDisplayName == "" && username != "" {
			item.UserDisplayName = "@" + username
		}
		if item.UserDisplayName == "" {
			item.UserDisplayName = fmt.Sprintf("User %d", item.Transaction.UserID)
		}
		pending = append(pending, item)
	}
	return pending, rows.Err()
}

func (s *PostgresStore) ApprovePendingDeposit(ctx context.Context, transactionID, settlementAsset string) (domain.Transaction, error) {
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return domain.Transaction{}, err
	}
	defer tx.Rollback()

	var transaction domain.Transaction
	var assetAmount string
	var settlementAmount string
	var price string
	var counterparty sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT id, kind, user_id, counterparty_user_id, asset_code, asset_amount, settlement_asset_code, settlement_amount, price, reference, status, description, created_at
		FROM transactions
		WHERE id = $1
		FOR UPDATE
	`, transactionID).Scan(
		&transaction.ID,
		&transaction.Kind,
		&transaction.UserID,
		&counterparty,
		&transaction.AssetCode,
		&assetAmount,
		&transaction.SettlementAsset,
		&settlementAmount,
		&price,
		&transaction.Reference,
		&transaction.Status,
		&transaction.Description,
		&transaction.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Transaction{}, ErrNotFound
		}
		return domain.Transaction{}, err
	}

	if transaction.Status != "pending" || transaction.Kind != domain.TransactionDeposit {
		return domain.Transaction{}, ErrInvalidStatus
	}

	transaction.AssetAmount, err = decimal.NewFromString(assetAmount)
	if err != nil {
		return domain.Transaction{}, err
	}
	transaction.SettlementAmount, err = decimal.NewFromString(settlementAmount)
	if err != nil {
		return domain.Transaction{}, err
	}
	transaction.Price, err = decimal.NewFromString(price)
	if err != nil {
		return domain.Transaction{}, err
	}
	if counterparty.Valid {
		value := counterparty.Int64
		transaction.CounterpartyID = &value
	}

	if err := s.incrementBalance(ctx, tx, transaction.UserID, settlementAsset, transaction.AssetAmount); err != nil {
		return domain.Transaction{}, err
	}

	transaction.Status = "success"
	transaction.SettlementAsset = settlementAsset
	transaction.SettlementAmount = transaction.AssetAmount

	_, err = tx.ExecContext(ctx, `
		UPDATE transactions
		SET status = 'success', settlement_asset_code = $2, settlement_amount = $3, description = $4
		WHERE id = $1
	`, transaction.ID, settlementAsset, transaction.AssetAmount.String(), "Deposit approved and TMN credited")
	if err != nil {
		return domain.Transaction{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Transaction{}, err
	}
	return transaction, nil
}

func (s *PostgresStore) Buy(ctx context.Context, userID int64, quote domain.TradeQuote) (domain.Transaction, error) {
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return domain.Transaction{}, err
	}
	defer tx.Rollback()

	if err := s.ensureBalanceRow(ctx, tx, userID, quote.SettlementAsset); err != nil {
		return domain.Transaction{}, err
	}
	if err := s.ensureBalanceRow(ctx, tx, userID, quote.Asset); err != nil {
		return domain.Transaction{}, err
	}
	current, err := s.lockBalance(ctx, tx, userID, quote.SettlementAsset)
	if err != nil {
		return domain.Transaction{}, err
	}
	if current.LessThan(quote.SettlementAmount) {
		return domain.Transaction{}, ErrInsufficientFunds
	}
	if err := s.setBalance(ctx, tx, userID, quote.SettlementAsset, current.Sub(quote.SettlementAmount)); err != nil {
		return domain.Transaction{}, err
	}
	assetBalance, err := s.lockBalance(ctx, tx, userID, quote.Asset)
	if err != nil {
		return domain.Transaction{}, err
	}
	if err := s.setBalance(ctx, tx, userID, quote.Asset, assetBalance.Add(quote.AssetAmount)); err != nil {
		return domain.Transaction{}, err
	}

	transaction, err := s.insertTransaction(ctx, tx, domain.Transaction{
		Kind:             domain.TransactionBuy,
		UserID:           userID,
		AssetCode:        quote.Asset,
		AssetAmount:      quote.AssetAmount,
		SettlementAsset:  quote.SettlementAsset,
		SettlementAmount: quote.SettlementAmount,
		Price:            quote.PriceInSettlement,
		Status:           "completed",
		Description:      fmt.Sprintf("Bought %s with %s", quote.Asset, quote.SettlementAsset),
	})
	if err != nil {
		return domain.Transaction{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Transaction{}, err
	}
	return transaction, nil
}

func (s *PostgresStore) Sell(ctx context.Context, userID int64, quote domain.TradeQuote) (domain.Transaction, error) {
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return domain.Transaction{}, err
	}
	defer tx.Rollback()

	if err := s.ensureBalanceRow(ctx, tx, userID, quote.SettlementAsset); err != nil {
		return domain.Transaction{}, err
	}
	if err := s.ensureBalanceRow(ctx, tx, userID, quote.Asset); err != nil {
		return domain.Transaction{}, err
	}
	assetBalance, err := s.lockBalance(ctx, tx, userID, quote.Asset)
	if err != nil {
		return domain.Transaction{}, err
	}
	if assetBalance.LessThan(quote.AssetAmount) {
		return domain.Transaction{}, ErrInsufficientFunds
	}
	if err := s.setBalance(ctx, tx, userID, quote.Asset, assetBalance.Sub(quote.AssetAmount)); err != nil {
		return domain.Transaction{}, err
	}
	current, err := s.lockBalance(ctx, tx, userID, quote.SettlementAsset)
	if err != nil {
		return domain.Transaction{}, err
	}
	if err := s.setBalance(ctx, tx, userID, quote.SettlementAsset, current.Add(quote.SettlementAmount)); err != nil {
		return domain.Transaction{}, err
	}

	transaction, err := s.insertTransaction(ctx, tx, domain.Transaction{
		Kind:             domain.TransactionSell,
		UserID:           userID,
		AssetCode:        quote.Asset,
		AssetAmount:      quote.AssetAmount,
		SettlementAsset:  quote.SettlementAsset,
		SettlementAmount: quote.SettlementAmount,
		Price:            quote.PriceInSettlement,
		Status:           "completed",
		Description:      fmt.Sprintf("Sold %s for %s", quote.Asset, quote.SettlementAsset),
	})
	if err != nil {
		return domain.Transaction{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Transaction{}, err
	}
	return transaction, nil
}

func (s *PostgresStore) Transfer(ctx context.Context, senderUserID, recipientUserID int64, asset string, amount decimal.Decimal) (domain.Transaction, error) {
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return domain.Transaction{}, err
	}
	defer tx.Rollback()

	if err := s.ensureBalanceRow(ctx, tx, senderUserID, asset); err != nil {
		return domain.Transaction{}, err
	}
	if err := s.ensureBalanceRow(ctx, tx, recipientUserID, asset); err != nil {
		return domain.Transaction{}, err
	}

	senderBalance, err := s.lockBalance(ctx, tx, senderUserID, asset)
	if err != nil {
		return domain.Transaction{}, err
	}
	if senderBalance.LessThan(amount) {
		return domain.Transaction{}, ErrInsufficientFunds
	}
	recipientBalance, err := s.lockBalance(ctx, tx, recipientUserID, asset)
	if err != nil {
		return domain.Transaction{}, err
	}
	if err := s.setBalance(ctx, tx, senderUserID, asset, senderBalance.Sub(amount)); err != nil {
		return domain.Transaction{}, err
	}
	if err := s.setBalance(ctx, tx, recipientUserID, asset, recipientBalance.Add(amount)); err != nil {
		return domain.Transaction{}, err
	}

	senderTx, err := s.insertTransaction(ctx, tx, domain.Transaction{
		Kind:            domain.TransactionTransferOut,
		UserID:          senderUserID,
		CounterpartyID:  &recipientUserID,
		AssetCode:       asset,
		AssetAmount:     amount,
		SettlementAsset: asset,
		Status:          "completed",
		Description:     "Internal transfer sent",
	})
	if err != nil {
		return domain.Transaction{}, err
	}
	_, err = s.insertTransaction(ctx, tx, domain.Transaction{
		Kind:            domain.TransactionTransferIn,
		UserID:          recipientUserID,
		CounterpartyID:  &senderUserID,
		AssetCode:       asset,
		AssetAmount:     amount,
		SettlementAsset: asset,
		Status:          "completed",
		Description:     "Internal transfer received",
	})
	if err != nil {
		return domain.Transaction{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Transaction{}, err
	}
	return senderTx, nil
}

func (s *PostgresStore) getUserByID(ctx context.Context, userID int64) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRowContext(ctx, `
		SELECT id, telegram_user_id, chat_id, username, first_name, last_name, share_code, locale, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&user.ID,
		&user.TelegramUserID,
		&user.ChatID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.ShareCode,
		&user.Locale,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func (s *PostgresStore) getUserByShareCode(ctx context.Context, shareCode string) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRowContext(ctx, `
		SELECT id, telegram_user_id, chat_id, username, first_name, last_name, share_code, locale, created_at, updated_at
		FROM users
		WHERE share_code = $1
	`, shareCode).Scan(
		&user.ID,
		&user.TelegramUserID,
		&user.ChatID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.ShareCode,
		&user.Locale,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) listBalances(ctx context.Context, userID int64, assets []string) ([]domain.Balance, error) {
	rows, err := s.pool.QueryContext(ctx, `
		SELECT asset_code, amount
		FROM balances
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := map[string]decimal.Decimal{}
	for rows.Next() {
		var asset string
		var amount string
		if err := rows.Scan(&asset, &amount); err != nil {
			return nil, err
		}
		values[asset], err = decimal.NewFromString(amount)
		if err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	balances := make([]domain.Balance, 0, len(assets))
	for _, asset := range assets {
		balances = append(balances, domain.Balance{
			Asset:     asset,
			Available: values[asset],
		})
	}
	return balances, nil
}

func (s *PostgresStore) listContacts(ctx context.Context, userID int64) ([]domain.Contact, error) {
	rows, err := s.pool.QueryContext(ctx, `
		SELECT c.contact_user_id, c.alias, u.first_name, u.last_name, u.username, u.share_code, c.created_at
		FROM contacts c
		JOIN users u ON u.id = c.contact_user_id
		WHERE c.owner_user_id = $1
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []domain.Contact
	for rows.Next() {
		var contact domain.Contact
		var firstName string
		var lastName string
		if err := rows.Scan(
			&contact.ContactUserID,
			&contact.Alias,
			&firstName,
			&lastName,
			&contact.Username,
			&contact.ShareCode,
			&contact.CreatedAt,
		); err != nil {
			return nil, err
		}
		contact.DisplayName = strings.TrimSpace(firstName + " " + lastName)
		if contact.DisplayName == "" && contact.Username != "" {
			contact.DisplayName = "@" + contact.Username
		}
		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

func (s *PostgresStore) listTransactions(ctx context.Context, userID int64, limit int) ([]domain.Transaction, error) {
	rows, err := s.pool.QueryContext(ctx, `
		SELECT id, kind, user_id, counterparty_user_id, asset_code, asset_amount, settlement_asset_code, settlement_amount, price, reference, status, description, created_at
		FROM transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []domain.Transaction
	for rows.Next() {
		var tx domain.Transaction
		var counterparty sql.NullInt64
		var assetAmount string
		var settlementAmount string
		var price string
		var reference string
		if err := rows.Scan(
			&tx.ID,
			&tx.Kind,
			&tx.UserID,
			&counterparty,
			&tx.AssetCode,
			&assetAmount,
			&tx.SettlementAsset,
			&settlementAmount,
			&price,
			&reference,
			&tx.Status,
			&tx.Description,
			&tx.CreatedAt,
		); err != nil {
			return nil, err
		}
		if counterparty.Valid {
			value := counterparty.Int64
			tx.CounterpartyID = &value
		}
		tx.Reference = reference
		tx.AssetAmount, err = decimal.NewFromString(assetAmount)
		if err != nil {
			return nil, err
		}
		tx.SettlementAmount, err = decimal.NewFromString(settlementAmount)
		if err != nil {
			return nil, err
		}
		tx.Price, err = decimal.NewFromString(price)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}
	return transactions, rows.Err()
}

func (s *PostgresStore) ensureBalanceRow(ctx context.Context, tx *sql.Tx, userID int64, asset string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO balances (user_id, asset_code, amount)
		VALUES ($1, $2, 0)
		ON CONFLICT (user_id, asset_code) DO NOTHING
	`, userID, asset)
	return err
}

func (s *PostgresStore) incrementBalance(ctx context.Context, tx *sql.Tx, userID int64, asset string, amount decimal.Decimal) error {
	if err := s.ensureBalanceRow(ctx, tx, userID, asset); err != nil {
		return err
	}
	current, err := s.lockBalance(ctx, tx, userID, asset)
	if err != nil {
		return err
	}
	return s.setBalance(ctx, tx, userID, asset, current.Add(amount))
}

func (s *PostgresStore) lockBalance(ctx context.Context, tx *sql.Tx, userID int64, asset string) (decimal.Decimal, error) {
	var amount string
	err := tx.QueryRowContext(ctx, `
		SELECT amount
		FROM balances
		WHERE user_id = $1 AND asset_code = $2
		FOR UPDATE
	`, userID, asset).Scan(&amount)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromString(amount)
}

func (s *PostgresStore) setBalance(ctx context.Context, tx *sql.Tx, userID int64, asset string, amount decimal.Decimal) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE balances
		SET amount = $3, updated_at = NOW()
		WHERE user_id = $1 AND asset_code = $2
	`, userID, asset, amount.String())
	return err
}

func (s *PostgresStore) insertTransaction(ctx context.Context, tx *sql.Tx, input domain.Transaction) (domain.Transaction, error) {
	input.ID = newTransactionID()
	err := tx.QueryRowContext(ctx, `
		INSERT INTO transactions (
			id, kind, user_id, counterparty_user_id, asset_code, asset_amount,
			settlement_asset_code, settlement_amount, price, reference, status, description
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at
	`,
		input.ID,
		input.Kind,
		input.UserID,
		input.CounterpartyID,
		input.AssetCode,
		input.AssetAmount.String(),
		input.SettlementAsset,
		input.SettlementAmount.String(),
		input.Price.String(),
		input.Reference,
		input.Status,
		input.Description,
	).Scan(&input.CreatedAt)
	if err != nil {
		return domain.Transaction{}, err
	}
	return input, nil
}

func newShareCode() string {
	return "EX-" + randomUpperString(8)
}

func newTransactionID() string {
	return "tx_" + randomUpperString(16)
}

func randomUpperString(n int) string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, n)
	randomBytes := make([]byte, n)
	if _, err := rand.Read(randomBytes); err != nil {
		panic(err)
	}
	for i := range buf {
		buf[i] = alphabet[int(randomBytes[i])%len(alphabet)]
	}
	return string(buf)
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
	id BIGSERIAL PRIMARY KEY,
	telegram_user_id BIGINT NOT NULL UNIQUE,
	chat_id BIGINT NOT NULL,
	username TEXT NOT NULL DEFAULT '',
	first_name TEXT NOT NULL DEFAULT '',
	last_name TEXT NOT NULL DEFAULT '',
	share_code TEXT NOT NULL UNIQUE,
	locale TEXT NOT NULL DEFAULT 'fa',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS balances (
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	asset_code TEXT NOT NULL,
	amount NUMERIC(30, 12) NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (user_id, asset_code)
);

CREATE TABLE IF NOT EXISTS contacts (
	owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	contact_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	alias TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (owner_user_id, contact_user_id),
	CHECK (owner_user_id <> contact_user_id)
);

CREATE TABLE IF NOT EXISTS transactions (
	id TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	counterparty_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
	asset_code TEXT NOT NULL,
	asset_amount NUMERIC(30, 12) NOT NULL DEFAULT 0,
	settlement_asset_code TEXT NOT NULL DEFAULT '',
	settlement_amount NUMERIC(30, 12) NOT NULL DEFAULT 0,
	price NUMERIC(30, 12) NOT NULL DEFAULT 0,
	reference TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'completed',
	description TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS app_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS reference TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS locale TEXT NOT NULL DEFAULT 'fa';
`
