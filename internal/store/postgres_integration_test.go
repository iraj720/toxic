package store

import (
	"context"
	"os"
	"testing"

	"github.com/shopspring/decimal"

	"exchange/internal/domain"
)

func TestPostgresStoreSmoke(t *testing.T) {
	databaseURL := os.Getenv("EXCHANGE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("EXCHANGE_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	store, err := New(ctx, databaseURL, "TMN", []string{"BTC", "TMN"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer store.Close()

	if _, err := store.pool.ExecContext(ctx, `
		DROP TABLE IF EXISTS transactions;
		DROP TABLE IF EXISTS contacts;
		DROP TABLE IF EXISTS balances;
		DROP TABLE IF EXISTS app_settings;
		DROP TABLE IF EXISTS users;
	`); err != nil {
		t.Fatalf("failed to reset schema: %v", err)
	}

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	user, created, err := store.EnsureUser(ctx, domain.TelegramProfile{
		TelegramUserID: 101,
		ChatID:         101,
		FirstName:      "Test",
		LastName:       "User",
	})
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if !created {
		t.Fatalf("expected user to be created")
	}

	tx, err := store.Deposit(ctx, user.ID, "TMN", decimal.NewFromInt(500))
	if err != nil {
		t.Fatalf("Deposit returned error: %v", err)
	}
	if tx.AssetAmount.String() != "500" {
		t.Fatalf("expected deposit amount 500, got %s", tx.AssetAmount)
	}

	dashboard, err := store.GetDashboard(ctx, user.ID, []string{"BTC", "TMN"})
	if err != nil {
		t.Fatalf("GetDashboard returned error: %v", err)
	}
	if len(dashboard.Balances) != 2 {
		t.Fatalf("expected 2 balances, got %d", len(dashboard.Balances))
	}
	if dashboard.Balances[1].Available.String() != "500" {
		t.Fatalf("expected TMN balance 500, got %s", dashboard.Balances[1].Available)
	}

	pending, err := store.CreatePendingDeposit(ctx, user.ID, "TMN", decimal.NewFromInt(250), "file-1", "Pending deposit")
	if err != nil {
		t.Fatalf("CreatePendingDeposit returned error: %v", err)
	}
	if pending.Status != "pending" {
		t.Fatalf("expected pending status, got %s", pending.Status)
	}

	approved, err := store.ApprovePendingDeposit(ctx, pending.ID, "TMN")
	if err != nil {
		t.Fatalf("ApprovePendingDeposit returned error: %v", err)
	}
	if approved.Status != "success" {
		t.Fatalf("expected success status, got %s", approved.Status)
	}

	dashboard, err = store.GetDashboard(ctx, user.ID, []string{"BTC", "TMN"})
	if err != nil {
		t.Fatalf("GetDashboard returned error after approval: %v", err)
	}
	if dashboard.Balances[1].Available.String() != "750" {
		t.Fatalf("expected TMN balance 750, got %s", dashboard.Balances[1].Available)
	}

	rate, ok, err := store.GetQuoteToSettlementRate(ctx)
	if err != nil {
		t.Fatalf("GetQuoteToSettlementRate returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected no stored rate override, got %s", rate)
	}

	if err := store.SetQuoteToSettlementRate(ctx, decimal.NewFromInt(56000)); err != nil {
		t.Fatalf("SetQuoteToSettlementRate returned error: %v", err)
	}

	rate, ok, err = store.GetQuoteToSettlementRate(ctx)
	if err != nil {
		t.Fatalf("GetQuoteToSettlementRate after set returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected stored rate override to exist")
	}
	if !rate.Equal(decimal.NewFromInt(56000)) {
		t.Fatalf("expected stored rate 56000, got %s", rate)
	}
}
