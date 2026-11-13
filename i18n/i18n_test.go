package i18n

import (
	"strings"
	"testing"
)

func TestEveryLocaleIsComplete(t *testing.T) {
	for _, locale := range Locales() {
		if missing := MissingKeys(locale); len(missing) > 0 {
			t.Errorf("locale %s is missing %v", locale, missing)
		}
	}
}

func TestEveryLocaleHasEveryReason(t *testing.T) {
	verdicts := []string{"proven", "not_proven", "conflicting"}
	for _, locale := range Locales() {
		for _, v := range verdicts {
			if Reason(locale, v, "a", "b") == "" {
				t.Errorf("locale %s has no reason for %s", locale, v)
			}
		}
	}
}

func TestReasonsCarryTheirValues(t *testing.T) {
	got := Reason("en", "proven", "-0.1234", "0.0100")
	if !strings.Contains(got, "-0.1234") || !strings.Contains(got, "0.0100") {
		t.Errorf("reason = %q", got)
	}
}

func TestFallbackToEnglish(t *testing.T) {
	if got := T("xx", "verdict"); got != T("en", "verdict") {
		t.Errorf("unknown locale fallback = %q", got)
	}
	if got := T("xx", "verdict_proven"); got != "PROVEN" {
		t.Errorf("unknown locale fallback = %q", got)
	}
}

func TestUnknownKeyReturnsTheKey(t *testing.T) {
	if got := T("en", "no_such_key"); got != "no_such_key" {
		t.Errorf("T = %q", got)
	}
}

func TestLocalesAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, locale := range Locales() {
		if seen[locale] {
			t.Errorf("duplicate locale %s", locale)
		}
		seen[locale] = true
	}
}

func TestVerdictLabelsDiffer(t *testing.T) {
	// The four shipped languages must not collapse onto the same words.
	for _, key := range []string{"verdict_proven", "verdict_not_proven", "title", "candidate"} {
		values := map[string]bool{}
		for _, locale := range Locales() {
			values[T(locale, key)] = true
		}
		if len(values) < 3 {
			t.Errorf("key %s has only %d distinct translations", key, len(values))
		}
	}
}

func TestHasAndUnknown(t *testing.T) {
	if !Has("ru") {
		t.Error("ru should be shipped")
	}
	if Has("xx") {
		t.Error("xx should not be shipped")
	}
	if got := Unknown([]string{"en", "xx", "ru"}); len(got) != 1 || got[0] != "xx" {
		t.Errorf("Unknown = %v", got)
	}
}
