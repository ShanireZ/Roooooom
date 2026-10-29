package i18n

import (
	"regexp"
	"slices"
	"testing"
)

var verbs = regexp.MustCompile(`%[sdv]`)

func TestCatalogParity(t *testing.T) {
	if len(messages[EN]) == 0 {
		t.Fatal("English catalog is empty")
	}
	for _, l := range []Locale{ZHCN, ZHTW, JA} {
		if len(messages[l]) != len(messages[EN]) {
			t.Errorf("%s has %d keys; English has %d", l, len(messages[l]), len(messages[EN]))
		}
		for k, en := range messages[EN] {
			text, ok := messages[l][k]
			if !ok || text == "" {
				t.Errorf("%s is missing %q", l, k)
				continue
			}
			if !slices.Equal(verbs.FindAllString(text, -1), verbs.FindAllString(en, -1)) {
				t.Errorf("%s %q verbs differ from English", l, k)
			}
		}
	}
}

func TestTFormatsAndFallsBack(t *testing.T) {
	if got := T(EN, TrayOpen); got != messages[EN][TrayOpen] {
		t.Fatalf("T(en, tray.open) = %q", got)
	}
	if got := T("xx", TrayOpen); got != messages[EN][TrayOpen] {
		t.Fatalf("unknown locale did not fall back to English: %q", got)
	}
	if got := T(EN, Key("missing.key")); got != "missing.key" {
		t.Fatalf("missing key = %q", got)
	}
}
