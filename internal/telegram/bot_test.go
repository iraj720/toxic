package telegram

import (
	"testing"

	"github.com/shopspring/decimal"

	"exchange/internal/config"
	"exchange/internal/localization"
)

func TestTMNWordsLineEnglish(t *testing.T) {
	bot := &Bot{
		cfg: config.Config{
			Market: config.MarketConfig{SettlementCurrency: "TMN"},
		},
		i18n: localization.New(),
	}

	line := bot.tmnWordsLine(localization.LocaleEnglish, "tmn_words.amount", decimal.NewFromInt(1), "TMN")
	want := "Amount in words: 1 thousand toman"
	if line != want {
		t.Fatalf("tmnWordsLine() = %q, want %q", line, want)
	}
}

func TestTMNWordsLinePersian(t *testing.T) {
	bot := &Bot{
		cfg: config.Config{
			Market: config.MarketConfig{SettlementCurrency: "TMN"},
		},
		i18n: localization.New(),
	}

	line := bot.tmnWordsLine(localization.LocalePersian, "tmn_words.amount", decimal.NewFromInt(1000), "TMN")
	want := "مبلغ به حروف: ۱ میلیون تومان"
	if line != want {
		t.Fatalf("tmnWordsLine() = %q, want %q", line, want)
	}
}

func TestTMNWordsLineSkipsNonTMNAssets(t *testing.T) {
	bot := &Bot{i18n: localization.New()}

	line := bot.tmnWordsLine(localization.LocaleEnglish, "tmn_words.amount", decimal.NewFromInt(5), "BTC")
	if line != "" {
		t.Fatalf("tmnWordsLine() = %q, want empty string", line)
	}
}

func TestDisplayAssetAddsTMNHint(t *testing.T) {
	bot := &Bot{i18n: localization.New()}

	if got := bot.displayAsset(localization.LocalePersian, "TMN"); got != "TMN (هزار تومن)" {
		t.Fatalf("displayAsset(fa, TMN) = %q", got)
	}
	if got := bot.displayAsset(localization.LocaleEnglish, "TMN"); got != "TMN (1,000 toman)" {
		t.Fatalf("displayAsset(en, TMN) = %q", got)
	}
	if got := bot.displayAsset(localization.LocalePersian, "BTC"); got != "BTC" {
		t.Fatalf("displayAsset(fa, BTC) = %q", got)
	}
}
