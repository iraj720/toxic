package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"exchange/internal/config"
	"exchange/internal/domain"
	"exchange/internal/localization"
	"exchange/internal/service"
	"exchange/internal/store"
)

const (
	stepLanguageSelect      = "language_select"
	stepCoinSelect          = "coin_select"
	stepCoinAction          = "coin_action"
	stepBuyAmount           = "buy_amount"
	stepBuyConfirm          = "buy_confirm"
	stepSellAmount          = "sell_amount"
	stepSellConfirm         = "sell_confirm"
	stepDepositAmount       = "deposit_amount"
	stepDepositReceipt      = "deposit_receipt"
	stepWithdrawAmount      = "withdraw_amount"
	stepWithdrawConfirm     = "withdraw_confirm"
	stepAddContact          = "add_contact"
	stepTransferRecipient   = "transfer_recipient"
	stepTransferAsset       = "transfer_asset"
	stepTransferAmount      = "transfer_amount"
	stepTransferConfirm     = "transfer_confirm"
	stepAdminApprovePick    = "admin_approve_pick"
	stepAdminApproveConfirm = "admin_approve_confirm"
	stepAdminRateAmount     = "admin_rate_amount"
	stepAdminRateConfirm    = "admin_rate_confirm"
)

type Bot struct {
	logger  *log.Logger
	cfg     config.Config
	client  *Client
	service *service.Service
	i18n    *localization.Localizer
}

func NewBot(logger *log.Logger, cfg config.Config, client *Client, svc *service.Service, i18n *localization.Localizer) *Bot {
	return &Bot{
		logger:  logger,
		cfg:     cfg,
		client:  client,
		service: svc,
		i18n:    i18n,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	if err := b.client.DeleteWebhook(ctx, true); err != nil {
		return fmt.Errorf("disable webhook: %w", err)
	}

	var offset int64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, err := b.client.GetUpdates(ctx, offset, b.cfg.Telegram.PollTimeoutSeconds)
		if err != nil {
			b.logger.Printf("get updates: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, update := range updates {
			offset = update.UpdateID + 1
			if update.Message == nil || update.Message.Chat.Type != "private" {
				continue
			}
			if err := b.handleMessage(ctx, update.Message); err != nil {
				b.logger.Printf("handle message: %v", err)
				_ = b.client.SendMessage(ctx, update.Message.Chat.ID, b.i18n.Text(localization.LocalePersian, "app.generic_error"), b.mainMenu(localization.LocalePersian, false))
			}
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *Message) error {
	user, created, err := b.service.EnsureUser(ctx, domain.TelegramProfile{
		TelegramUserID: msg.From.ID,
		ChatID:         msg.Chat.ID,
		Username:       msg.From.Username,
		FirstName:      msg.From.FirstName,
		LastName:       msg.From.LastName,
	})
	if err != nil {
		return err
	}

	locale := b.locale(user.Locale)
	text := messageText(msg)

	if code, ok := parseAuthCommand(text); ok {
		if b.service.AuthenticateAdmin(code) {
			b.service.Sessions().AuthorizeAdmin(user.ID)
			return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "auth.success"), b.mainMenu(locale, true))
		}
		return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "auth.invalid"), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
	}

	if created {
		return b.client.SendMessage(
			ctx,
			msg.Chat.ID,
			b.i18n.Text(locale, "welcome.message", user.DisplayName(), user.ShareCode, b.cfg.Market.SettlementCurrency),
			b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)),
		)
	}

	if isMenuReset(text) {
		b.service.Sessions().Clear(user.ID)
		return b.sendMainMenu(ctx, msg.Chat.ID, locale, b.service.Sessions().IsAdmin(user.ID))
	}

	if session, ok := b.service.Sessions().Get(user.ID); ok {
		handled, err := b.handleSession(ctx, msg.Chat.ID, user, msg, session)
		if err != nil || handled {
			return err
		}
	}

	if text == "" && len(msg.Photo) == 0 {
		return b.sendMainMenu(ctx, msg.Chat.ID, locale, b.service.Sessions().IsAdmin(user.ID))
	}

	switch {
	case b.i18n.Matches(text, "cancel"):
		b.service.Sessions().Clear(user.ID)
		return b.sendMainMenu(ctx, msg.Chat.ID, locale, b.service.Sessions().IsAdmin(user.ID))
	case b.i18n.Matches(text, "profile"):
		return b.sendProfile(ctx, msg.Chat.ID, user.TelegramUserID)
	case b.i18n.Matches(text, "contacts"):
		return b.sendContacts(ctx, msg.Chat.ID, user.TelegramUserID)
	case b.i18n.Matches(text, "coins"):
		b.service.Sessions().Set(user.ID, service.Session{Step: stepCoinSelect, Data: map[string]string{}})
		return b.sendCoins(ctx, msg.Chat.ID, user.Locale)
	case b.i18n.Matches(text, "transfer"):
		b.service.Sessions().Set(user.ID, service.Session{Step: stepTransferRecipient, Data: map[string]string{}})
		return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "transfer.ask_recipient"), b.singleActionMenu(locale, "cancel"))
	case b.i18n.Matches(text, "share_contact"):
		return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "share_contact.message", user.ShareCode), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
	case b.i18n.Matches(text, "deposit"):
		if b.service.DepositCardNumber() == "" {
			return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "deposit.card_missing"), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
		}
		b.service.Sessions().Set(user.ID, service.Session{Step: stepDepositAmount, Data: map[string]string{}})
		return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "deposit.ask_amount", b.cfg.Market.SettlementCurrency), b.singleActionMenu(locale, "cancel"))
	case b.i18n.Matches(text, "withdraw"):
		b.service.Sessions().Set(user.ID, service.Session{Step: stepWithdrawAmount, Data: map[string]string{}})
		return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "withdraw.ask_amount", b.cfg.Market.SettlementCurrency), b.singleActionMenu(locale, "cancel"))
	case b.i18n.Matches(text, "transaction_history"):
		return b.sendTransactions(ctx, msg.Chat.ID, user.TelegramUserID)
	case b.i18n.Matches(text, "add_contact"):
		b.service.Sessions().Set(user.ID, service.Session{Step: stepAddContact, Data: map[string]string{}})
		return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "contacts.add.ask"), b.singleActionMenu(locale, "cancel"))
	case b.i18n.Matches(text, "language"):
		b.service.Sessions().Set(user.ID, service.Session{Step: stepLanguageSelect, Data: map[string]string{}})
		return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "language.choose"), b.languageMenu(locale))
	case b.i18n.Matches(text, "admin_panel"):
		if !b.service.Sessions().IsAdmin(user.ID) {
			return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "admin.unauthorized"), b.mainMenu(locale, false))
		}
		return b.sendAdminPanel(ctx, msg.Chat.ID, locale)
	case b.i18n.Matches(text, "pending_deposits"):
		if !b.service.Sessions().IsAdmin(user.ID) {
			return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "admin.unauthorized"), b.mainMenu(locale, false))
		}
		return b.sendPendingDeposits(ctx, msg.Chat.ID, user.ID, locale)
	case b.i18n.Matches(text, "set_rate"):
		if !b.service.Sessions().IsAdmin(user.ID) {
			return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "admin.unauthorized"), b.mainMenu(locale, false))
		}
		b.service.Sessions().Set(user.ID, service.Session{Step: stepAdminRateAmount, Data: map[string]string{}})
		return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "admin.rate_prompt", b.cfg.Market.QuoteCurrency, b.cfg.Market.SettlementCurrency), b.singleActionMenu(locale, "cancel"))
	case b.i18n.Matches(text, "logout_admin"):
		b.service.Sessions().RevokeAdmin(user.ID)
		b.service.Sessions().Clear(user.ID)
		return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "admin.logout"), b.mainMenu(locale, false))
	default:
		return b.client.SendMessage(ctx, msg.Chat.ID, b.i18n.Text(locale, "app.unknown_action"), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
	}
}

func (b *Bot) handleSession(ctx context.Context, chatID int64, user domain.User, msg *Message, session service.Session) (bool, error) {
	locale := b.locale(user.Locale)
	text := messageText(msg)

	if b.i18n.Matches(text, "cancel") {
		b.service.Sessions().Clear(user.ID)
		switch session.Step {
		case stepLanguageSelect, stepAddContact:
			return true, b.sendProfile(ctx, chatID, user.TelegramUserID)
		case stepAdminApprovePick, stepAdminApproveConfirm, stepAdminRateAmount, stepAdminRateConfirm:
			return true, b.sendAdminPanel(ctx, chatID, locale)
		default:
			return true, b.sendMainMenu(ctx, chatID, locale, b.service.Sessions().IsAdmin(user.ID))
		}
	}

	switch session.Step {
	case stepLanguageSelect:
		var nextLocale string
		switch {
		case b.i18n.Matches(text, "persian"):
			nextLocale = localization.LocalePersian
		case b.i18n.Matches(text, "english"):
			nextLocale = localization.LocaleEnglish
		default:
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "language.choose"), b.languageMenu(locale))
		}
		if err := b.service.SetLocale(ctx, user.TelegramUserID, nextLocale); err != nil {
			return true, err
		}
		b.service.Sessions().Clear(user.ID)
		if err := b.client.SendMessage(ctx, chatID, b.i18n.Text(nextLocale, "language.changed"), b.profileMenu(nextLocale, b.service.Sessions().IsAdmin(user.ID))); err != nil {
			return true, err
		}
		return true, b.sendProfile(ctx, chatID, user.TelegramUserID)
	case stepCoinSelect:
		asset := strings.ToUpper(strings.TrimSpace(text))
		if !contains(b.service.EnabledCoinSymbols(), asset) {
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "coins.invalid_choice"), b.coinSelectionMenu(locale))
		}
		session.Step = stepCoinAction
		session.Data["asset"] = asset
		b.service.Sessions().Set(user.ID, session)
		return true, b.sendCoinActions(ctx, chatID, asset, locale)
	case stepCoinAction:
		switch {
		case b.i18n.Matches(text, "buy"):
			session.Step = stepBuyAmount
			b.service.Sessions().Set(user.ID, session)
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "buy.ask_amount", b.cfg.Market.SettlementCurrency, session.Data["asset"]), b.singleActionMenu(locale, "cancel"))
		case b.i18n.Matches(text, "sell"):
			session.Step = stepSellAmount
			b.service.Sessions().Set(user.ID, session)
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "sell.ask_amount", session.Data["asset"]), b.singleActionMenu(locale, "cancel"))
		default:
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "coin.choose_action"), b.coinActionMenu(locale))
		}
	case stepBuyAmount:
		amount, err := parsePositiveDecimal(text)
		if err != nil {
			return true, b.client.SendMessage(ctx, chatID, b.validationMessage(locale, err), b.singleActionMenu(locale, "cancel"))
		}
		quote, err := b.service.QuoteBuy(ctx, session.Data["asset"], amount)
		if err != nil {
			return true, err
		}
		session.Step = stepBuyConfirm
		session.Data["asset_amount"] = quote.AssetAmount.String()
		session.Data["gross_settlement_amount"] = quote.GrossSettlementAmount.String()
		session.Data["settlement_amount"] = quote.SettlementAmount.String()
		session.Data["fee_amount"] = quote.FeeAmount.String()
		session.Data["price"] = quote.PriceInSettlement.String()
		b.service.Sessions().Set(user.ID, session)
		return true, b.client.SendMessage(
			ctx,
			chatID,
			b.i18n.Text(locale, "buy.review", quote.Asset, formatAmount(quote.PriceInSettlement, quote.SettlementAsset), quote.SettlementAsset, formatAmount(quote.GrossSettlementAmount, quote.SettlementAsset), quote.SettlementAsset, formatAmount(quote.FeeAmount, quote.FeeAsset), quote.FeeAsset, formatAmount(quote.SettlementAmount, quote.SettlementAsset), quote.SettlementAsset, formatAmount(quote.AssetAmount, quote.Asset), quote.Asset, quote.Source),
			b.confirmMenu(locale),
		)
	case stepBuyConfirm:
		if !b.i18n.Matches(text, "confirm") {
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "buy.confirm_prompt"), b.confirmMenu(locale))
		}
		quote := domain.TradeQuote{
			Asset:                 session.Data["asset"],
			AssetAmount:           decimal.RequireFromString(session.Data["asset_amount"]),
			SettlementAsset:       b.cfg.Market.SettlementCurrency,
			GrossSettlementAmount: decimal.RequireFromString(session.Data["gross_settlement_amount"]),
			SettlementAmount:      decimal.RequireFromString(session.Data["settlement_amount"]),
			FeeAsset:              b.cfg.Market.SettlementCurrency,
			FeeAmount:             decimal.RequireFromString(session.Data["fee_amount"]),
			PriceInSettlement:     decimal.RequireFromString(session.Data["price"]),
		}
		tx, err := b.service.Buy(ctx, user.TelegramUserID, quote)
		if err != nil {
			if errors.Is(err, store.ErrInsufficientFunds) {
				return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "buy.insufficient", b.cfg.Market.SettlementCurrency), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
			}
			return true, err
		}
		b.service.Sessions().Clear(user.ID)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "buy.success", tx.ID, formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, formatAmount(tx.FeeAmount, tx.FeeAssetCode), tx.FeeAssetCode, formatAmount(tx.SettlementAmount, tx.SettlementAsset), tx.SettlementAsset), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
	case stepSellAmount:
		amount, err := parsePositiveDecimal(text)
		if err != nil {
			return true, b.client.SendMessage(ctx, chatID, b.validationMessage(locale, err), b.singleActionMenu(locale, "cancel"))
		}
		quote, err := b.service.QuoteSell(ctx, session.Data["asset"], amount)
		if err != nil {
			return true, err
		}
		session.Step = stepSellConfirm
		session.Data["asset_amount"] = quote.AssetAmount.String()
		session.Data["gross_settlement_amount"] = quote.GrossSettlementAmount.String()
		session.Data["settlement_amount"] = quote.SettlementAmount.String()
		session.Data["fee_amount"] = quote.FeeAmount.String()
		session.Data["price"] = quote.PriceInSettlement.String()
		b.service.Sessions().Set(user.ID, session)
		return true, b.client.SendMessage(
			ctx,
			chatID,
			b.i18n.Text(locale, "sell.review", quote.Asset, formatAmount(quote.PriceInSettlement, quote.SettlementAsset), quote.SettlementAsset, formatAmount(quote.AssetAmount, quote.Asset), quote.Asset, formatAmount(quote.GrossSettlementAmount, quote.SettlementAsset), quote.SettlementAsset, formatAmount(quote.FeeAmount, quote.FeeAsset), quote.FeeAsset, formatAmount(quote.SettlementAmount, quote.SettlementAsset), quote.SettlementAsset, quote.Source),
			b.confirmMenu(locale),
		)
	case stepSellConfirm:
		if !b.i18n.Matches(text, "confirm") {
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "sell.confirm_prompt"), b.confirmMenu(locale))
		}
		quote := domain.TradeQuote{
			Asset:                 session.Data["asset"],
			AssetAmount:           decimal.RequireFromString(session.Data["asset_amount"]),
			SettlementAsset:       b.cfg.Market.SettlementCurrency,
			GrossSettlementAmount: decimal.RequireFromString(session.Data["gross_settlement_amount"]),
			SettlementAmount:      decimal.RequireFromString(session.Data["settlement_amount"]),
			FeeAsset:              b.cfg.Market.SettlementCurrency,
			FeeAmount:             decimal.RequireFromString(session.Data["fee_amount"]),
			PriceInSettlement:     decimal.RequireFromString(session.Data["price"]),
		}
		tx, err := b.service.Sell(ctx, user.TelegramUserID, quote)
		if err != nil {
			if errors.Is(err, store.ErrInsufficientFunds) {
				return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "sell.insufficient", quote.Asset), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
			}
			return true, err
		}
		b.service.Sessions().Clear(user.ID)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "sell.success", tx.ID, formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, formatAmount(tx.FeeAmount, tx.FeeAssetCode), tx.FeeAssetCode, formatAmount(tx.SettlementAmount, tx.SettlementAsset), tx.SettlementAsset), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
	case stepDepositAmount:
		amount, err := parsePositiveDecimal(text)
		if err != nil {
			return true, b.client.SendMessage(ctx, chatID, b.validationMessage(locale, err), b.singleActionMenu(locale, "cancel"))
		}
		session.Step = stepDepositReceipt
		session.Data["amount"] = amount.String()
		b.service.Sessions().Set(user.ID, session)
		lines := []string{
			b.i18n.Text(locale, "deposit.instructions", formatAmount(amount, b.cfg.Market.SettlementCurrency), b.cfg.Market.SettlementCurrency, b.service.DepositCardNumber()),
		}
		if helper := b.tmnHelper(locale); helper != "" {
			lines = append(lines, "", helper)
		}
		return true, b.client.SendMessage(ctx, chatID, strings.Join(lines, "\n"), b.singleActionMenu(locale, "cancel"))
	case stepDepositReceipt:
		if len(msg.Photo) == 0 {
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "deposit.ask_receipt"), b.singleActionMenu(locale, "cancel"))
		}
		receiptFileID := msg.Photo[len(msg.Photo)-1].FileID
		tx, err := b.service.CreatePendingDeposit(ctx, user.TelegramUserID, decimal.RequireFromString(session.Data["amount"]), receiptFileID)
		if err != nil {
			return true, err
		}
		b.service.Sessions().Clear(user.ID)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "deposit.pending_success", tx.ID, formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, b.localizeStatus(locale, tx.Status)), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
	case stepWithdrawAmount:
		amount, err := parsePositiveDecimal(text)
		if err != nil {
			return true, b.client.SendMessage(ctx, chatID, b.validationMessage(locale, err), b.singleActionMenu(locale, "cancel"))
		}
		session.Step = stepWithdrawConfirm
		session.Data["amount"] = amount.String()
		b.service.Sessions().Set(user.ID, session)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "withdraw.review", formatAmount(amount, b.cfg.Market.SettlementCurrency), b.cfg.Market.SettlementCurrency), b.confirmMenu(locale))
	case stepWithdrawConfirm:
		if !b.i18n.Matches(text, "confirm") {
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "withdraw.confirm_prompt"), b.confirmMenu(locale))
		}
		b.service.Sessions().Clear(user.ID)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "withdraw.unavailable", formatAmount(decimal.RequireFromString(session.Data["amount"]), b.cfg.Market.SettlementCurrency), b.cfg.Market.SettlementCurrency), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
	case stepAddContact:
		contact, err := b.service.AddContact(ctx, user.TelegramUserID, text)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "contacts.add.not_found"), b.contactsMenu(locale))
			}
			return true, err
		}
		b.service.Sessions().Clear(user.ID)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "contacts.add.success", contact.DisplayName, contact.ShareCode), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
	case stepTransferRecipient:
		recipient, err := b.service.ResolveRecipient(ctx, user.TelegramUserID, text)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "transfer.recipient_missing"), b.singleActionMenu(locale, "cancel"))
			}
			return true, err
		}
		session.Step = stepTransferAsset
		session.Data["recipient_id"] = strconv.FormatInt(recipient.ID, 10)
		session.Data["recipient_name"] = recipient.DisplayName()
		b.service.Sessions().Set(user.ID, session)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "transfer.recipient_selected", recipient.DisplayName()), b.coinSelectionMenu(locale))
	case stepTransferAsset:
		asset := strings.ToUpper(strings.TrimSpace(text))
		if !contains(b.service.EnabledCoinSymbols(), asset) {
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "transfer.choose_coin"), b.coinSelectionMenu(locale))
		}
		session.Step = stepTransferAmount
		session.Data["asset"] = asset
		b.service.Sessions().Set(user.ID, session)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "transfer.ask_amount", asset), b.singleActionMenu(locale, "cancel"))
	case stepTransferAmount:
		amount, err := parsePositiveDecimal(text)
		if err != nil {
			return true, b.client.SendMessage(ctx, chatID, b.validationMessage(locale, err), b.singleActionMenu(locale, "cancel"))
		}
		quote, err := b.service.QuoteTransfer(session.Data["asset"], amount)
		if err != nil {
			return true, err
		}
		session.Step = stepTransferConfirm
		session.Data["amount"] = quote.AssetAmount.String()
		session.Data["fee_amount"] = quote.FeeAmount.String()
		session.Data["total_debit_amount"] = quote.TotalDebitAmount.String()
		b.service.Sessions().Set(user.ID, session)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "transfer.review", session.Data["recipient_name"], quote.Asset, formatAmount(quote.AssetAmount, quote.Asset), quote.Asset, formatAmount(quote.FeeAmount, quote.FeeAsset), quote.FeeAsset, formatAmount(quote.TotalDebitAmount, quote.Asset), quote.Asset), b.confirmMenu(locale))
	case stepTransferConfirm:
		if !b.i18n.Matches(text, "confirm") {
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "transfer.confirm_prompt"), b.confirmMenu(locale))
		}
		recipientID, err := strconv.ParseInt(session.Data["recipient_id"], 10, 64)
		if err != nil {
			return true, err
		}
		tx, err := b.service.Transfer(ctx, user.TelegramUserID, recipientID, domain.TransferQuote{
			Asset:            session.Data["asset"],
			AssetAmount:      decimal.RequireFromString(session.Data["amount"]),
			FeeAsset:         session.Data["asset"],
			FeeAmount:        decimal.RequireFromString(session.Data["fee_amount"]),
			TotalDebitAmount: decimal.RequireFromString(session.Data["total_debit_amount"]),
		})
		if err != nil {
			if errors.Is(err, store.ErrInsufficientFunds) {
				return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "transfer.insufficient", session.Data["asset"]), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
			}
			return true, err
		}
		b.service.Sessions().Clear(user.ID)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "transfer.success", tx.ID, session.Data["recipient_name"], formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, formatAmount(tx.FeeAmount, tx.FeeAssetCode), tx.FeeAssetCode, formatAmount(tx.SettlementAmount, tx.SettlementAsset), tx.SettlementAsset), b.mainMenu(locale, b.service.Sessions().IsAdmin(user.ID)))
	case stepAdminApprovePick:
		if !b.service.Sessions().IsAdmin(user.ID) {
			b.service.Sessions().Clear(user.ID)
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.unauthorized"), b.mainMenu(locale, false))
		}
		pending, err := b.service.ListPendingDeposits(ctx, 50)
		if err != nil {
			return true, err
		}
		for _, item := range pending {
			if item.Transaction.ID != text {
				continue
			}
			session.Step = stepAdminApproveConfirm
			session.Data["transaction_id"] = item.Transaction.ID
			session.Data["user_display_name"] = item.UserDisplayName
			session.Data["amount"] = item.Transaction.AssetAmount.String()
			b.service.Sessions().Set(user.ID, session)
			caption := b.i18n.Text(locale, "admin.approve_review", item.Transaction.ID, item.UserDisplayName, formatAmount(item.Transaction.AssetAmount, item.Transaction.AssetCode), item.Transaction.AssetCode, b.localizeStatus(locale, item.Transaction.Status))
			if strings.TrimSpace(item.Transaction.Reference) != "" {
				if err := b.client.SendPhoto(ctx, chatID, item.Transaction.Reference, caption, b.confirmMenu(locale)); err == nil {
					return true, nil
				}
			}
			return true, b.client.SendMessage(ctx, chatID, caption, b.confirmMenu(locale))
		}
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.invalid_pending"), b.adminDepositSelectionMenu(locale, pending))
	case stepAdminApproveConfirm:
		if !b.service.Sessions().IsAdmin(user.ID) {
			b.service.Sessions().Clear(user.ID)
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.unauthorized"), b.mainMenu(locale, false))
		}
		if !b.i18n.Matches(text, "confirm") {
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.approve_confirm"), b.confirmMenu(locale))
		}
		tx, err := b.service.ApprovePendingDeposit(ctx, session.Data["transaction_id"])
		if err != nil {
			if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrInvalidStatus) {
				return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.approve_unavailable"), b.adminPanelMenu(locale))
			}
			return true, err
		}
		b.service.Sessions().Clear(user.ID)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.approve_success", tx.ID, session.Data["user_display_name"], formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, b.localizeStatus(locale, tx.Status)), b.adminPanelMenu(locale))
	case stepAdminRateAmount:
		if !b.service.Sessions().IsAdmin(user.ID) {
			b.service.Sessions().Clear(user.ID)
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.unauthorized"), b.mainMenu(locale, false))
		}
		rate, err := parsePositiveDecimal(text)
		if err != nil {
			return true, b.client.SendMessage(ctx, chatID, b.validationMessage(locale, err), b.singleActionMenu(locale, "cancel"))
		}
		session.Step = stepAdminRateConfirm
		session.Data["rate"] = rate.String()
		b.service.Sessions().Set(user.ID, session)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.rate_review", b.cfg.Market.QuoteCurrency, formatAmount(rate, b.cfg.Market.SettlementCurrency), b.cfg.Market.SettlementCurrency), b.confirmMenu(locale))
	case stepAdminRateConfirm:
		if !b.service.Sessions().IsAdmin(user.ID) {
			b.service.Sessions().Clear(user.ID)
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.unauthorized"), b.mainMenu(locale, false))
		}
		if !b.i18n.Matches(text, "confirm") {
			return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.rate_confirm_prompt"), b.confirmMenu(locale))
		}
		currentRate, err := b.service.SetQuoteToSettlementRate(ctx, decimal.RequireFromString(session.Data["rate"]))
		if err != nil {
			return true, err
		}
		b.service.Sessions().Clear(user.ID)
		return true, b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.rate_success", currentRate.QuoteCurrency, formatAmount(currentRate.Rate, currentRate.SettlementCurrency), currentRate.SettlementCurrency, b.localizeRateSource(locale, currentRate.Source)), b.adminPanelMenu(locale))
	default:
		b.service.Sessions().Clear(user.ID)
		return false, nil
	}
}

func (b *Bot) sendProfile(ctx context.Context, chatID int64, telegramUserID int64) error {
	dashboard, err := b.service.Dashboard(ctx, telegramUserID)
	if err != nil {
		return err
	}
	locale := b.locale(dashboard.User.Locale)

	var settlementBalance decimal.Decimal
	var holdings []string
	for _, balance := range dashboard.Balances {
		if balance.Asset == b.cfg.Market.SettlementCurrency {
			settlementBalance = balance.Available
			continue
		}
		if balance.Available.GreaterThan(decimal.Zero) {
			holdings = append(holdings, fmt.Sprintf("- %s %s", formatAmount(balance.Available, balance.Asset), balance.Asset))
		}
	}

	lines := []string{
		b.i18n.Text(locale, "profile.summary", dashboard.User.DisplayName(), dashboard.User.ShareCode, b.cfg.Market.SettlementCurrency, formatAmount(settlementBalance, b.cfg.Market.SettlementCurrency), b.cfg.Market.SettlementCurrency),
	}
	if helper := b.tmnHelper(locale); helper != "" {
		lines = append(lines, helper)
	}
	if len(holdings) > 0 {
		lines = append(lines, "", b.i18n.Text(locale, "profile.holdings_header"))
		lines = append(lines, holdings...)
	} else {
		lines = append(lines, "", b.i18n.Text(locale, "profile.no_holdings"))
	}

	return b.client.SendMessage(ctx, chatID, strings.Join(lines, "\n"), b.profileMenu(locale, b.service.Sessions().IsAdmin(dashboard.User.ID)))
}

func (b *Bot) sendCoins(ctx context.Context, chatID int64, locale string) error {
	locale = b.locale(locale)
	prices, err := b.service.ListMarketPrices(ctx)
	if err != nil {
		return err
	}
	rate, err := b.service.CurrentQuoteToSettlementRate(ctx)
	if err != nil {
		return err
	}

	lines := []string{
		b.i18n.Text(locale, "coins.header"),
		"",
		b.i18n.Text(locale, "coins.rate", rate.QuoteCurrency, formatAmount(rate.Rate, rate.SettlementCurrency), rate.SettlementCurrency, b.localizeRateSource(locale, rate.Source)),
		"",
	}
	for _, price := range prices {
		lines = append(lines, fmt.Sprintf("- %s: %s %s | %s %s", price.Asset, formatAmount(price.PriceInQuote, price.QuoteCurrency), price.QuoteCurrency, formatAmount(price.PriceInSettlement, price.SettlementCurrency), price.SettlementCurrency))
	}
	lines = append(lines, "", b.i18n.Text(locale, "coins.prompt"))
	return b.client.SendMessage(ctx, chatID, strings.Join(lines, "\n"), b.coinSelectionMenu(locale))
}

func (b *Bot) sendCoinActions(ctx context.Context, chatID int64, asset, locale string) error {
	locale = b.locale(locale)
	price, err := b.service.CoinPrice(ctx, asset)
	if err != nil {
		return err
	}
	return b.client.SendMessage(
		ctx,
		chatID,
		b.i18n.Text(locale, "coin.actions", asset, formatAmount(price.PriceInQuote, price.QuoteCurrency), price.QuoteCurrency, price.SettlementCurrency, formatAmount(price.PriceInSettlement, price.SettlementCurrency), price.SettlementCurrency),
		b.coinActionMenu(locale),
	)
}

func (b *Bot) sendContacts(ctx context.Context, chatID int64, telegramUserID int64) error {
	dashboard, err := b.service.Dashboard(ctx, telegramUserID)
	if err != nil {
		return err
	}
	locale := b.locale(dashboard.User.Locale)

	lines := []string{b.i18n.Text(locale, "contacts.header"), ""}
	if len(dashboard.Contacts) == 0 {
		lines = append(lines, b.i18n.Text(locale, "contacts.none"))
	} else {
		for _, contact := range dashboard.Contacts {
			label := contact.DisplayName
			if label == "" {
				label = contact.Alias
			}
			lines = append(lines, fmt.Sprintf("- %s | %s", label, contact.ShareCode))
		}
	}
	lines = append(lines, "", b.i18n.Text(locale, "contacts.prompt"))
	return b.client.SendMessage(ctx, chatID, strings.Join(lines, "\n"), b.contactsMenu(locale))
}

func (b *Bot) sendTransactions(ctx context.Context, chatID int64, telegramUserID int64) error {
	dashboard, err := b.service.Dashboard(ctx, telegramUserID)
	if err != nil {
		return err
	}
	locale := b.locale(dashboard.User.Locale)

	lines := []string{b.i18n.Text(locale, "history.header"), ""}
	if len(dashboard.Transactions) == 0 {
		lines = append(lines, b.i18n.Text(locale, "history.none"))
	} else {
		for _, tx := range dashboard.Transactions {
			lines = append(lines, b.formatTransaction(locale, tx))
		}
	}

	return b.client.SendMessage(ctx, chatID, strings.Join(lines, "\n"), b.mainMenu(locale, b.service.Sessions().IsAdmin(dashboard.User.ID)))
}

func (b *Bot) sendAdminPanel(ctx context.Context, chatID int64, locale string) error {
	rate, err := b.service.CurrentQuoteToSettlementRate(ctx)
	if err != nil {
		return err
	}

	lines := []string{
		b.i18n.Text(locale, "admin.panel"),
		"",
		b.i18n.Text(locale, "admin.current_rate", rate.QuoteCurrency, formatAmount(rate.Rate, rate.SettlementCurrency), rate.SettlementCurrency, b.localizeRateSource(locale, rate.Source)),
	}
	return b.client.SendMessage(ctx, chatID, strings.Join(lines, "\n"), b.adminPanelMenu(locale))
}

func (b *Bot) sendPendingDeposits(ctx context.Context, chatID int64, userID int64, locale string) error {
	locale = b.locale(locale)
	pending, err := b.service.ListPendingDeposits(ctx, 20)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "admin.no_pending"), b.adminPanelMenu(locale))
	}

	lines := []string{b.i18n.Text(locale, "admin.pending_header"), ""}
	for _, item := range pending {
		lines = append(lines, fmt.Sprintf("- %s | %s | %s %s", item.Transaction.ID, item.UserDisplayName, formatAmount(item.Transaction.AssetAmount, item.Transaction.AssetCode), item.Transaction.AssetCode))
	}
	lines = append(lines, "", b.i18n.Text(locale, "admin.pending_prompt"))
	b.service.Sessions().Set(userID, service.Session{Step: stepAdminApprovePick, Data: map[string]string{}})
	return b.client.SendMessage(ctx, chatID, strings.Join(lines, "\n"), b.adminDepositSelectionMenu(locale, pending))
}

func (b *Bot) mainMenu(locale string, isAdmin bool) *ReplyKeyboardMarkup {
	rows := [][]string{
		{b.i18n.Button(locale, "profile"), b.i18n.Button(locale, "coins")},
		{b.i18n.Button(locale, "transfer"), b.i18n.Button(locale, "share_contact")},
		{b.i18n.Button(locale, "deposit"), b.i18n.Button(locale, "withdraw")},
		{b.i18n.Button(locale, "transaction_history")},
	}
	if isAdmin {
		rows = append(rows, []string{b.i18n.Button(locale, "admin_panel")})
	}
	return keyboard(rows...)
}

func (b *Bot) sendMainMenu(ctx context.Context, chatID int64, locale string, isAdmin bool) error {
	locale = b.locale(locale)
	return b.client.SendMessage(ctx, chatID, b.i18n.Text(locale, "app.main_menu"), b.mainMenu(locale, isAdmin))
}

func (b *Bot) profileMenu(locale string, isAdmin bool) *ReplyKeyboardMarkup {
	rows := [][]string{
		{b.i18n.Button(locale, "contacts"), b.i18n.Button(locale, "language")},
		{b.i18n.Button(locale, "cancel")},
	}
	if isAdmin {
		rows = append(rows, []string{b.i18n.Button(locale, "admin_panel")})
	}
	return keyboard(rows...)
}

func (b *Bot) coinSelectionMenu(locale string) *ReplyKeyboardMarkup {
	coins := append([]string{}, b.service.EnabledCoinSymbols()...)
	coins = append(coins, b.i18n.Button(locale, "cancel"))
	return chunkedKeyboard(coins, 2)
}

func (b *Bot) coinActionMenu(locale string) *ReplyKeyboardMarkup {
	return keyboard(
		[]string{b.i18n.Button(locale, "buy"), b.i18n.Button(locale, "sell")},
		[]string{b.i18n.Button(locale, "cancel")},
	)
}

func (b *Bot) contactsMenu(locale string) *ReplyKeyboardMarkup {
	return keyboard(
		[]string{b.i18n.Button(locale, "add_contact")},
		[]string{b.i18n.Button(locale, "cancel")},
	)
}

func (b *Bot) languageMenu(locale string) *ReplyKeyboardMarkup {
	return keyboard(
		[]string{b.i18n.Button(locale, "persian"), b.i18n.Button(locale, "english")},
		[]string{b.i18n.Button(locale, "cancel")},
	)
}

func (b *Bot) adminPanelMenu(locale string) *ReplyKeyboardMarkup {
	return keyboard(
		[]string{b.i18n.Button(locale, "pending_deposits"), b.i18n.Button(locale, "set_rate")},
		[]string{b.i18n.Button(locale, "logout_admin")},
		[]string{b.i18n.Button(locale, "cancel")},
	)
}

func (b *Bot) adminDepositSelectionMenu(locale string, items []domain.PendingDeposit) *ReplyKeyboardMarkup {
	labels := make([]string, 0, len(items)+1)
	for _, item := range items {
		labels = append(labels, item.Transaction.ID)
	}
	labels = append(labels, b.i18n.Button(locale, "cancel"))
	return chunkedKeyboard(labels, 1)
}

func (b *Bot) confirmMenu(locale string) *ReplyKeyboardMarkup {
	return keyboard([]string{b.i18n.Button(locale, "confirm"), b.i18n.Button(locale, "cancel")})
}

func (b *Bot) singleActionMenu(locale, key string) *ReplyKeyboardMarkup {
	return keyboard([]string{b.i18n.Button(locale, key)})
}

func keyboard(rows ...[]string) *ReplyKeyboardMarkup {
	keyboardRows := make([][]KeyboardButton, 0, len(rows))
	for _, row := range rows {
		buttons := make([]KeyboardButton, 0, len(row))
		for _, label := range row {
			buttons = append(buttons, KeyboardButton{Text: label})
		}
		keyboardRows = append(keyboardRows, buttons)
	}
	return &ReplyKeyboardMarkup{
		Keyboard:       keyboardRows,
		ResizeKeyboard: true,
		IsPersistent:   true,
	}
}

func chunkedKeyboard(labels []string, chunkSize int) *ReplyKeyboardMarkup {
	var rows [][]string
	for i := 0; i < len(labels); i += chunkSize {
		end := i + chunkSize
		if end > len(labels) {
			end = len(labels)
		}
		rows = append(rows, labels[i:end])
	}
	return keyboard(rows...)
}

func messageText(msg *Message) string {
	if strings.TrimSpace(msg.Text) != "" {
		return strings.TrimSpace(msg.Text)
	}
	return strings.TrimSpace(msg.Caption)
}

func parseAuthCommand(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(strings.ToLower(input), "/auth ") {
		return "", false
	}
	return strings.TrimSpace(input[len("/auth "):]), true
}

func isMenuReset(input string) bool {
	normalized := strings.ToLower(strings.TrimSpace(input))
	return normalized == "/start" || normalized == "menu"
}

func parsePositiveDecimal(input string) (decimal.Decimal, error) {
	value, err := decimal.NewFromString(strings.TrimSpace(input))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid_number")
	}
	if value.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("amount_positive")
	}
	return value, nil
}

func formatAmount(amount decimal.Decimal, asset string) string {
	precision := int32(8)
	if asset == "TMN" || asset == "USDT" {
		precision = 2
	}
	return amount.Round(precision).StringFixedBank(precision)
}

func (b *Bot) locale(locale string) string {
	return b.i18n.Normalize(locale)
}

func (b *Bot) localizeStatus(locale, status string) string {
	key := "status." + strings.ToLower(strings.TrimSpace(status))
	return b.i18n.Text(locale, key)
}

func (b *Bot) localizeRateSource(locale, source string) string {
	key := "rate_source." + strings.ToLower(strings.TrimSpace(source))
	return b.i18n.Text(locale, key)
}

func (b *Bot) validationMessage(locale string, err error) string {
	switch err.Error() {
	case "invalid_number":
		return b.i18n.Text(locale, "app.invalid_number")
	case "amount_positive":
		return b.i18n.Text(locale, "app.amount_positive")
	default:
		return err.Error()
	}
}

func (b *Bot) tmnHelper(locale string) string {
	if strings.ToUpper(strings.TrimSpace(b.cfg.Market.SettlementCurrency)) != "TMN" {
		return ""
	}
	return b.i18n.Text(locale, "tmn.helper")
}

func (b *Bot) formatTransaction(locale string, tx domain.Transaction) string {
	timestamp := tx.CreatedAt.Local().Format("2006-01-02 15:04")
	status := b.localizeStatus(locale, tx.Status)
	fee := ""
	if tx.FeeAmount.GreaterThan(decimal.Zero) && strings.TrimSpace(tx.FeeAssetCode) != "" {
		fee = fmt.Sprintf(" | %s %s %s", b.i18n.Text(locale, "tx.fee"), formatAmount(tx.FeeAmount, tx.FeeAssetCode), tx.FeeAssetCode)
	}
	switch tx.Kind {
	case domain.TransactionBuy:
		return fmt.Sprintf("- %s | %s | %s %s | %s %s %s%s | %s", timestamp, b.i18n.Text(locale, "tx.buy"), formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, b.i18n.Text(locale, "tx.spent"), formatAmount(tx.SettlementAmount, tx.SettlementAsset), tx.SettlementAsset, fee, status)
	case domain.TransactionSell:
		return fmt.Sprintf("- %s | %s | %s %s | %s %s %s%s | %s", timestamp, b.i18n.Text(locale, "tx.sell"), formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, b.i18n.Text(locale, "tx.received"), formatAmount(tx.SettlementAmount, tx.SettlementAsset), tx.SettlementAsset, fee, status)
	case domain.TransactionTransferOut:
		return fmt.Sprintf("- %s | %s | %s %s%s | %s", timestamp, b.i18n.Text(locale, "tx.transfer_sent"), formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, fee, status)
	case domain.TransactionTransferIn:
		return fmt.Sprintf("- %s | %s | %s %s | %s", timestamp, b.i18n.Text(locale, "tx.transfer_received"), formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, status)
	case domain.TransactionDeposit:
		return fmt.Sprintf("- %s | %s | %s %s | %s", timestamp, b.i18n.Text(locale, "tx.deposit"), formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, status)
	default:
		return fmt.Sprintf("- %s | %s | %s %s | %s", timestamp, tx.Kind, formatAmount(tx.AssetAmount, tx.AssetCode), tx.AssetCode, status)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
