package localization

import (
	"fmt"
	"strings"
)

const (
	LocalePersian = "fa"
	LocaleEnglish = "en"
)

type Localizer struct {
	messages map[string]map[string]string
}

func New() *Localizer {
	return &Localizer{
		messages: map[string]map[string]string{
			LocalePersian: persianMessages(),
			LocaleEnglish: englishMessages(),
		},
	}
}

func (l *Localizer) Normalize(locale string) string {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case LocaleEnglish:
		return LocaleEnglish
	default:
		return LocalePersian
	}
}

func (l *Localizer) Text(locale, key string, args ...any) string {
	locale = l.Normalize(locale)
	message := l.messages[locale][key]
	if message == "" {
		message = l.messages[LocaleEnglish][key]
	}
	if message == "" {
		message = key
	}
	if len(args) == 0 {
		return message
	}
	return fmt.Sprintf(message, args...)
}

func (l *Localizer) Button(locale, key string) string {
	return l.Text(locale, "button."+key)
}

func (l *Localizer) Matches(input string, keys ...string) bool {
	input = normalizeText(input)
	if input == "" {
		return false
	}
	for _, locale := range []string{LocalePersian, LocaleEnglish} {
		for _, key := range keys {
			if input == normalizeText(l.Button(locale, key)) {
				return true
			}
		}
	}
	for _, key := range keys {
		if input == normalizeText(key) {
			return true
		}
	}
	return false
}

func normalizeText(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func englishMessages() map[string]string {
	return map[string]string{
		"button.profile":             "Profile",
		"button.contacts":            "Contacts",
		"button.add_contact":         "Add Contact",
		"button.coins":               "Coins",
		"button.transfer":            "Transfer",
		"button.share_contact":       "Share Contact",
		"button.deposit":             "Deposit",
		"button.withdraw":            "Withdraw TMN",
		"button.transaction_history": "Transaction History",
		"button.admin_panel":         "Admin Panel",
		"button.pending_deposits":    "Pending Deposits",
		"button.set_rate":            "Set Rate",
		"button.logout_admin":        "Logout Admin",
		"button.cancel":              "Cancel",
		"button.buy":                 "Buy",
		"button.sell":                "Sell",
		"button.confirm":             "Confirm",
		"button.language":            "Language",
		"button.persian":             "فارسی",
		"button.english":             "English",

		"auth.success":                "Admin login successful.\n\nUse Admin Panel to review pending deposits.",
		"auth.invalid":                "Invalid admin auth code.",
		"admin.unauthorized":          "You are not authenticated as an admin.",
		"admin.logout":                "Admin session closed.",
		"admin.panel":                 "Admin Panel\n\n- Pending Deposits: review and approve submitted deposits\n- Set Rate: update the active quote-to-settlement conversion\n- Logout Admin: close the current admin session",
		"admin.current_rate":          "Current rate: 1 %s = %s %s (%s)",
		"admin.no_pending":            "There are no pending deposits right now.",
		"admin.pending_header":        "Pending Deposits",
		"admin.pending_prompt":        "Send the transaction ID you want to approve.",
		"admin.invalid_pending":       "That transaction is not in the pending deposit list.",
		"admin.approve_review":        "Approve deposit\n\nTransaction: %s\nUser: %s\nAmount: %s %s\nStatus: %s\n\nReply Confirm to approve and credit TMN.",
		"admin.approve_confirm":       "Reply Confirm to approve the deposit, or Cancel to stop.",
		"admin.approve_unavailable":   "That deposit can no longer be approved.",
		"admin.approve_success":       "Deposit approved.\n\nTransaction: %s\nUser: %s\nAmount credited: %s %s\nStatus: %s",
		"admin.rate_prompt":           "Send the new rate for 1 %s in %s.",
		"admin.rate_review":           "Rate review\n\n1 %s = %s %s\n\nReply Confirm to apply this rate.",
		"admin.rate_confirm_prompt":   "Reply Confirm to apply this rate, or Cancel to stop.",
		"admin.rate_success":          "Rate updated.\n\n1 %s = %s %s\nSource: %s",
		"app.generic_error":           "Something went wrong. Please try again.",
		"app.main_menu":               "Main menu",
		"app.tap_menu":                "Tap one of the menu buttons to continue.",
		"app.unknown_action":          "I did not recognize that action. Use the menu buttons below.",
		"app.invalid_number":          "Enter a valid number.",
		"app.amount_positive":         "Amount must be greater than zero.",
		"asset.tmn_suffix":            "(1,000 toman)",
		"tmn_words.amount":            "Amount in words",
		"tmn_words.trade_amount":      "Trade amount in words",
		"tmn_words.fee":               "Fee in words",
		"tmn_words.total_charge":      "Total charge in words",
		"tmn_words.gross_receive":     "Gross receive in words",
		"tmn_words.net_receive":       "Net receive in words",
		"tmn_words.total_debit":       "Total debit in words",
		"tmn_words.toman":             "toman",
		"tmn_words.thousand":          "thousand",
		"tmn_words.million":           "million",
		"tmn_words.billion":           "billion",
		"tmn_words.trillion":          "trillion",
		"welcome.message":             "Welcome %s.\n\nYour account is ready.\nShare code: %s\n\nUse the menu below to open your profile, review coins, deposit %s, transfer assets, and check your transaction history.",
		"profile.summary":             "Profile\n\nAccount: %s\nShare code: %s\nTotal %s: %s %s",
		"tmn.helper":                  "1 TMN means 1,000 toman.",
		"profile.holdings_header":     "Coin holdings:",
		"profile.no_holdings":         "You do not hold any coins yet.",
		"language.choose":             "Choose your language.",
		"language.changed":            "Language updated.",
		"contacts.header":             "Contacts",
		"contacts.none":               "No contacts saved yet.",
		"contacts.prompt":             "Use Add Contact to save a new contact by share code.",
		"contacts.add.ask":            "Send the share code of the person you want to add. Example: EX-AB12CD34",
		"contacts.add.not_found":      "No user was found with that share code.",
		"contacts.add.success":        "Contact added.\n\nName: %s\nShare code: %s",
		"coins.header":                "Coins",
		"coins.rate":                  "Current rate: 1 %s = %s %s (%s)",
		"coins.prompt":                "Tap a coin below to buy or sell it.",
		"coins.invalid_choice":        "Choose one of the available coins.",
		"coin.actions":                "%s actions\n\nPrice: %s %s\nPrice in %s: %s %s\n\nChoose Buy or Sell.",
		"coin.choose_action":          "Choose Buy or Sell for this coin.",
		"buy.ask_amount":              "Enter how much %s you want to spend to buy %s.",
		"buy.review":                  "Buy review\n\nCoin: %s\nPrice: %s %s\nTrade amount: %s %s\nFee: %s %s\nTotal charge: %s %s\nReceive: %s %s\nSource: %s\n\nReply Confirm to execute.",
		"buy.confirm_prompt":          "Reply Confirm to execute the buy, or Cancel to stop.",
		"buy.insufficient":            "You do not have enough %s for this trade.",
		"buy.success":                 "Buy completed.\n\nTransaction: %s\nBought: %s %s\nFee: %s %s\nTotal charged: %s %s",
		"sell.ask_amount":             "Enter how much %s you want to sell.",
		"sell.review":                 "Sell review\n\nCoin: %s\nPrice: %s %s\nSell: %s %s\nGross receive: %s %s\nFee: %s %s\nNet receive: %s %s\nSource: %s\n\nReply Confirm to execute.",
		"sell.confirm_prompt":         "Reply Confirm to execute the sell, or Cancel to stop.",
		"sell.insufficient":           "You do not have enough %s to sell that amount.",
		"sell.success":                "Sell completed.\n\nTransaction: %s\nSold: %s %s\nFee: %s %s\nNet received: %s %s",
		"deposit.card_missing":        "Deposit card number is not configured yet.",
		"deposit.ask_amount":          "Enter the %s amount you want to deposit.",
		"deposit.instructions":        "Deposit instructions\n\nAmount: %s %s\nCard number: %s\n\nTransfer the amount to this card, then send the receipt photo here. Your deposit will be created in pending state until it is reviewed.",
		"deposit.ask_receipt":         "Send the receipt photo for your deposit, or Cancel to stop.",
		"deposit.pending_success":     "Deposit submitted.\n\nTransaction: %s\nAmount: %s %s\nStatus: %s\n\nYour receipt has been recorded and the deposit is now pending review.",
		"withdraw.ask_amount":         "Enter how much %s you want to withdraw.",
		"withdraw.review":             "Withdrawal review\n\nAsset: %s %s\n\nReply Confirm to continue.",
		"withdraw.confirm_prompt":     "Reply Confirm to continue with this withdrawal request, or Cancel to stop.",
		"withdraw.unavailable":        "Withdrawal request received for %s %s.\n\nFor now, because of current conditions, withdrawals are temporarily unavailable.",
		"transfer.ask_recipient":      "Send the recipient share code or an exact saved contact alias.",
		"transfer.recipient_missing":  "Recipient not found. Send a saved contact alias or a valid share code.",
		"transfer.recipient_selected": "Recipient selected: %s\n\nChoose the coin you want to transfer.",
		"transfer.choose_coin":        "Choose one of the available coins for the transfer.",
		"transfer.ask_amount":         "Enter how much %s you want to transfer.",
		"transfer.review":             "Transfer review\n\nRecipient: %s\nCoin: %s\nRecipient gets: %s %s\nFee: %s %s\nTotal debit: %s %s\n\nReply Confirm to send.",
		"transfer.confirm_prompt":     "Reply Confirm to execute the transfer, or Cancel to stop.",
		"transfer.insufficient":       "You do not have enough %s to complete that transfer.",
		"transfer.success":            "Transfer completed.\n\nTransaction: %s\nRecipient: %s\nSent: %s %s\nFee: %s %s\nTotal debited: %s %s",
		"tx.fee":                      "fee",
		"share_contact.message":       "Your share code is %s.\n\nShare this code with another bot user so they can add you and transfer assets to you.",
		"history.header":              "Transaction History",
		"history.none":                "No transactions yet.",
		"tx.buy":                      "buy",
		"tx.sell":                     "sell",
		"tx.transfer_sent":            "transfer sent",
		"tx.transfer_received":        "transfer received",
		"tx.deposit":                  "deposit",
		"tx.spent":                    "spent",
		"tx.received":                 "received",
		"status.pending":              "pending",
		"status.success":              "success",
		"status.completed":            "completed",
		"rate_source.config":          "default config",
		"rate_source.admin":           "admin override",
	}
}

func persianMessages() map[string]string {
	return map[string]string{
		"button.profile":             "پروفایل",
		"button.contacts":            "مخاطبین",
		"button.add_contact":         "افزودن مخاطب",
		"button.coins":               "کوین‌ها",
		"button.transfer":            "انتقال",
		"button.share_contact":       "اشتراک مخاطب",
		"button.deposit":             "واریز",
		"button.withdraw":            "برداشت TMN",
		"button.transaction_history": "تاریخچه تراکنش‌ها",
		"button.admin_panel":         "پنل ادمین",
		"button.pending_deposits":    "واریزهای در انتظار",
		"button.set_rate":            "تنظیم نرخ",
		"button.logout_admin":        "خروج ادمین",
		"button.cancel":              "بازگشت",
		"button.buy":                 "خرید",
		"button.sell":                "فروش",
		"button.confirm":             "تایید",
		"button.language":            "زبان",
		"button.persian":             "فارسی",
		"button.english":             "English",

		"auth.success":                "ورود ادمین با موفقیت انجام شد.\n\nبرای بررسی واریزهای در انتظار از پنل ادمین استفاده کنید.",
		"auth.invalid":                "کد ورود ادمین نامعتبر است.",
		"admin.unauthorized":          "شما به عنوان ادمین وارد نشده‌اید.",
		"admin.logout":                "نشست ادمین بسته شد.",
		"admin.panel":                 "پنل ادمین\n\n- واریزهای در انتظار: بررسی و تایید واریزها\n- تنظیم نرخ: به‌روزرسانی نرخ فعال تبدیل\n- خروج ادمین: بستن نشست فعلی ادمین",
		"admin.current_rate":          "نرخ فعلی: هر ۱ %s برابر با %s %s است (%s)",
		"admin.no_pending":            "در حال حاضر واریز در انتظاری وجود ندارد.",
		"admin.pending_header":        "واریزهای در انتظار",
		"admin.pending_prompt":        "شناسه تراکنشی را که می‌خواهید تایید کنید ارسال کنید.",
		"admin.invalid_pending":       "این تراکنش در فهرست واریزهای در انتظار نیست.",
		"admin.approve_review":        "تایید واریز\n\nتراکنش: %s\nکاربر: %s\nمبلغ: %s %s\nوضعیت: %s\n\nبرای تایید و شارژ TMN، دکمه تایید را بزنید.",
		"admin.approve_confirm":       "برای تایید واریز، دکمه تایید را بزنید یا بازگشت را انتخاب کنید.",
		"admin.approve_unavailable":   "این واریز دیگر قابل تایید نیست.",
		"admin.approve_success":       "واریز تایید شد.\n\nتراکنش: %s\nکاربر: %s\nمبلغ شارژ شده: %s %s\nوضعیت: %s",
		"admin.rate_prompt":           "نرخ جدید هر ۱ %s به %s را ارسال کنید.",
		"admin.rate_review":           "بازبینی نرخ\n\nهر ۱ %s = %s %s\n\nبرای اعمال این نرخ، دکمه تایید را بزنید.",
		"admin.rate_confirm_prompt":   "برای اعمال این نرخ، دکمه تایید را بزنید یا بازگشت را انتخاب کنید.",
		"admin.rate_success":          "نرخ با موفقیت به‌روزرسانی شد.\n\nهر ۱ %s = %s %s\nمنبع: %s",
		"app.generic_error":           "مشکلی پیش آمد. دوباره تلاش کنید.",
		"app.main_menu":               "منوی اصلی",
		"app.tap_menu":                "یکی از دکمه‌های منو را انتخاب کنید.",
		"app.unknown_action":          "این عملیات را متوجه نشدم. از دکمه‌های منو استفاده کنید.",
		"app.invalid_number":          "یک عدد معتبر وارد کنید.",
		"app.amount_positive":         "مبلغ باید بیشتر از صفر باشد.",
		"asset.tmn_suffix":            "(هزار تومن)",
		"tmn_words.amount":            "مبلغ به حروف",
		"tmn_words.trade_amount":      "مبلغ معامله به حروف",
		"tmn_words.fee":               "کارمزد به حروف",
		"tmn_words.total_charge":      "مبلغ کل پرداخت به حروف",
		"tmn_words.gross_receive":     "مبلغ ناخالص دریافتی به حروف",
		"tmn_words.net_receive":       "مبلغ خالص دریافتی به حروف",
		"tmn_words.total_debit":       "مجموع کسر از حساب به حروف",
		"tmn_words.toman":             "تومان",
		"tmn_words.thousand":          "هزار",
		"tmn_words.million":           "میلیون",
		"tmn_words.billion":           "میلیارد",
		"tmn_words.trillion":          "تریلیون",
		"welcome.message":             "خوش آمدید %s.\n\nحساب شما آماده است.\nکد اشتراک: %s\n\nاز منوی پایین برای مشاهده پروفایل، بررسی کوین‌ها، واریز %s، انتقال دارایی و دیدن تاریخچه تراکنش‌ها استفاده کنید.",
		"profile.summary":             "پروفایل\n\nحساب: %s\nکد اشتراک: %s\nجمع %s: %s %s",
		"tmn.helper":                  "هر 1 TMN یعنی هزار تومن.",
		"profile.holdings_header":     "دارایی کوین‌ها:",
		"profile.no_holdings":         "هنوز هیچ کوینی ندارید.",
		"language.choose":             "زبان را انتخاب کنید.",
		"language.changed":            "زبان با موفقیت تغییر کرد.",
		"contacts.header":             "مخاطبین",
		"contacts.none":               "هنوز مخاطبی ذخیره نشده است.",
		"contacts.prompt":             "برای ذخیره مخاطب جدید از دکمه افزودن مخاطب استفاده کنید.",
		"contacts.add.ask":            "کد اشتراک فرد مورد نظر را ارسال کنید. مثال: EX-AB12CD34",
		"contacts.add.not_found":      "کاربری با این کد اشتراک پیدا نشد.",
		"contacts.add.success":        "مخاطب اضافه شد.\n\nنام: %s\nکد اشتراک: %s",
		"coins.header":                "کوین‌ها",
		"coins.rate":                  "نرخ فعلی: هر ۱ %s برابر است با %s %s (%s)",
		"coins.prompt":                "برای خرید یا فروش، یکی از کوین‌ها را از پایین انتخاب کنید.",
		"coins.invalid_choice":        "یکی از کوین‌های موجود را انتخاب کنید.",
		"coin.actions":                "عملیات %s\n\nقیمت: %s %s\nقیمت به %s: %s %s\n\nخرید یا فروش را انتخاب کنید.",
		"coin.choose_action":          "برای این کوین خرید یا فروش را انتخاب کنید.",
		"buy.ask_amount":              "مقدار %s که می‌خواهید برای خرید %s خرج کنید را وارد کنید.",
		"buy.review":                  "بازبینی خرید\n\nکوین: %s\nقیمت: %s %s\nمبلغ معامله: %s %s\nکارمزد: %s %s\nمبلغ کل پرداخت: %s %s\nمقدار دریافتی: %s %s\nمنبع: %s\n\nبرای انجام خرید، دکمه تایید را بزنید.",
		"buy.confirm_prompt":          "برای انجام خرید، دکمه تایید را بزنید یا بازگشت را انتخاب کنید.",
		"buy.insufficient":            "موجودی %s شما برای این معامله کافی نیست.",
		"buy.success":                 "خرید انجام شد.\n\nتراکنش: %s\nخرید: %s %s\nکارمزد: %s %s\nمبلغ کل پرداخت: %s %s",
		"sell.ask_amount":             "مقدار %s که می‌خواهید بفروشید را وارد کنید.",
		"sell.review":                 "بازبینی فروش\n\nکوین: %s\nقیمت: %s %s\nمقدار فروش: %s %s\nمبلغ ناخالص دریافتی: %s %s\nکارمزد: %s %s\nمبلغ خالص دریافتی: %s %s\nمنبع: %s\n\nبرای انجام فروش، دکمه تایید را بزنید.",
		"sell.confirm_prompt":         "برای انجام فروش، دکمه تایید را بزنید یا بازگشت را انتخاب کنید.",
		"sell.insufficient":           "موجودی %s شما برای این فروش کافی نیست.",
		"sell.success":                "فروش انجام شد.\n\nتراکنش: %s\nفروش: %s %s\nکارمزد: %s %s\nمبلغ خالص دریافتی: %s %s",
		"deposit.card_missing":        "شماره کارت واریز هنوز تنظیم نشده است.",
		"deposit.ask_amount":          "مبلغ %s مورد نظر برای واریز را وارد کنید.",
		"deposit.instructions":        "راهنمای واریز\n\nمبلغ: %s %s\nشماره کارت: %s\n\nمبلغ را به این کارت واریز کنید و سپس عکس رسید را همین‌جا ارسال کنید. تراکنش واریز تا زمان بررسی در وضعیت در انتظار ثبت می‌شود.",
		"deposit.ask_receipt":         "عکس رسید واریز را ارسال کنید یا بازگشت را انتخاب کنید.",
		"deposit.pending_success":     "واریز ثبت شد.\n\nتراکنش: %s\nمبلغ: %s %s\nوضعیت: %s\n\nرسید شما ثبت شد و واریز اکنون در انتظار بررسی است.",
		"withdraw.ask_amount":         "مقدار %s که می‌خواهید برداشت کنید را وارد کنید.",
		"withdraw.review":             "بازبینی برداشت\n\nدارایی: %s %s\n\nبرای ادامه، دکمه تایید را بزنید.",
		"withdraw.confirm_prompt":     "برای ادامه درخواست برداشت، دکمه تایید را بزنید یا بازگشت را انتخاب کنید.",
		"withdraw.unavailable":        "درخواست برداشت برای %s %s ثبت شد.\n\nفعلاً به خاطر شرایط موجود، امکان برداشت در دسترس نیست.",
		"transfer.ask_recipient":      "کد اشتراک گیرنده یا نام دقیق یکی از مخاطبین ذخیره‌شده را ارسال کنید.",
		"transfer.recipient_missing":  "گیرنده پیدا نشد. کد اشتراک معتبر یا نام مخاطب ذخیره‌شده را ارسال کنید.",
		"transfer.recipient_selected": "گیرنده انتخاب شد: %s\n\nکوینی که می‌خواهید منتقل کنید را انتخاب کنید.",
		"transfer.choose_coin":        "یکی از کوین‌های موجود را برای انتقال انتخاب کنید.",
		"transfer.ask_amount":         "مقدار %s که می‌خواهید منتقل کنید را وارد کنید.",
		"transfer.review":             "بازبینی انتقال\n\nگیرنده: %s\nکوین: %s\nمقدار دریافتی گیرنده: %s %s\nکارمزد: %s %s\nمجموع کسر از حساب: %s %s\n\nبرای ارسال، دکمه تایید را بزنید.",
		"transfer.confirm_prompt":     "برای انجام انتقال، دکمه تایید را بزنید یا بازگشت را انتخاب کنید.",
		"transfer.insufficient":       "موجودی %s شما برای این انتقال کافی نیست.",
		"transfer.success":            "انتقال انجام شد.\n\nتراکنش: %s\nگیرنده: %s\nمقدار ارسال‌شده: %s %s\nکارمزد: %s %s\nمجموع کسر از حساب: %s %s",
		"tx.fee":                      "کارمزد",
		"share_contact.message":       "کد اشتراک شما: %s\n\nاین کد را با کاربر دیگری که از ربات استفاده می‌کند به اشتراک بگذارید تا بتواند شما را اضافه کند و برایتان انتقال انجام دهد.",
		"history.header":              "تاریخچه تراکنش‌ها",
		"history.none":                "هنوز تراکنشی ثبت نشده است.",
		"tx.buy":                      "خرید",
		"tx.sell":                     "فروش",
		"tx.transfer_sent":            "انتقال ارسالی",
		"tx.transfer_received":        "انتقال دریافتی",
		"tx.deposit":                  "واریز",
		"tx.spent":                    "پرداخت",
		"tx.received":                 "دریافت",
		"status.pending":              "در انتظار",
		"status.success":              "موفق",
		"status.completed":            "موفق",
		"rate_source.config":          "تنظیمات پیش‌فرض",
		"rate_source.admin":           "تغییر ادمین",
	}
}
