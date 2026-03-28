package localization

import "testing"

func TestNormalizeDefaultsToPersian(t *testing.T) {
	l := New()

	if got := l.Normalize(""); got != LocalePersian {
		t.Fatalf("expected empty locale to default to %q, got %q", LocalePersian, got)
	}
	if got := l.Normalize("unknown"); got != LocalePersian {
		t.Fatalf("expected unknown locale to default to %q, got %q", LocalePersian, got)
	}
	if got := l.Normalize("en"); got != LocaleEnglish {
		t.Fatalf("expected english locale, got %q", got)
	}
}

func TestTextFallsBackAcrossLocales(t *testing.T) {
	l := New()

	if got := l.Text("", "button.profile"); got != "پروفایل" {
		t.Fatalf("expected persian profile button, got %q", got)
	}
	if got := l.Text("en", "button.profile"); got != "Profile" {
		t.Fatalf("expected english profile button, got %q", got)
	}
}

func TestMatchesSupportsBothLocalizedButtons(t *testing.T) {
	l := New()

	if !l.Matches("پروفایل", "profile") {
		t.Fatal("expected persian button text to match profile key")
	}
	if !l.Matches("Profile", "profile") {
		t.Fatal("expected english button text to match profile key")
	}
	if !l.Matches("بازگشت", "cancel") {
		t.Fatal("expected persian cancel text to match cancel key")
	}
}
