package i18n

import "testing"

func TestParse(t *testing.T) {
	cases := map[string]Locale{
		"zh-TW": ZHTW, "zh-HK": ZHTW, "zh-MO": ZHTW, "zh-Hant": ZHTW, "zh-Hant-TW": ZHTW, "ZH_tw": ZHTW,
		"zh": ZHCN, "zh-CN": ZHCN, "zh-SG": ZHCN, "zh-Hans": ZHCN, "zh_cn": ZHCN,
		"ja": JA, "ja-JP": JA, "JA_jp": JA,
		"en": EN, "en-GB": EN, "EN_us": EN,
	}
	for tag, want := range cases {
		if got, ok := Parse(tag); !ok || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", tag, got, ok, want)
		}
	}
	for _, tag := range []string{"pt-BR", "", "xx", "zhx"} {
		if got, ok := Parse(tag); ok || got != "" {
			t.Errorf("Parse(%q) = %q, %v; want unsupported", tag, got, ok)
		}
	}
}

func TestFromAcceptLanguage(t *testing.T) {
	cases := map[string]Locale{
		"fr;q=0.9, ja;q=0.8":        JA,
		"en;q=0.1, zh-TW;q=0.9":     ZHTW,
		"zh-CN,zh;q=0.9,en;q=0.8":   ZHCN,
		"de, en-US;q=0.5, ja;q=0.4": EN,
	}
	for header, want := range cases {
		if got, ok := FromAcceptLanguage(header); !ok || got != want {
			t.Errorf("FromAcceptLanguage(%q) = %q, %v; want %q", header, got, ok, want)
		}
	}
	for _, header := range []string{"*", "", "fr, de;q=0.5", "ja;q=0"} {
		if got, ok := FromAcceptLanguage(header); ok || got != "" {
			t.Errorf("FromAcceptLanguage(%q) = %q, %v; want none", header, got, ok)
		}
	}
}

func TestFirst(t *testing.T) {
	if got := First("", ZHCN); got != ZHCN {
		t.Fatalf("First(\"\", zh-CN) = %q", got)
	}
	if got := First(); got != EN {
		t.Fatalf("First() = %q", got)
	}
	if got := First("xx"); got != EN {
		t.Fatalf("First(xx) = %q", got)
	}
	if got := First("xx", JA, ZHTW); got != JA {
		t.Fatalf("First(xx, ja, zh-TW) = %q", got)
	}
}

func TestOSReturnsSupportedLocaleOrNothing(t *testing.T) {
	l, ok := OS()
	if ok && First(l) != l {
		t.Fatalf("OS() = %q, which is not a supported locale", l)
	}
	t.Logf("OS() = %q, %v", l, ok)
}
