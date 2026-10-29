package launcher

import (
	"testing"

	"roooooom.local/roooooom/internal/i18n"
)

func TestTrayLabelsPerLocale(t *testing.T) {
	keys := []i18n.Key{i18n.TrayOpen, i18n.TrayReactivate, i18n.TrayExit, i18n.TrayStop}
	for _, l := range []i18n.Locale{i18n.EN, i18n.ZHCN, i18n.ZHTW, i18n.JA} {
		items := trayLabels(l)
		if len(items) != 4 {
			t.Fatalf("%s: %d items", l, len(items))
		}
		for i, id := range []uintptr{1, 4, 2, 3} {
			if items[i].id != id || items[i].label != i18n.T(l, keys[i]) {
				t.Errorf("%s item %d = %+v", l, i, items[i])
			}
		}
	}
	if got := trayLabels(i18n.ZHTW)[0].label; got != "開啟 Roooooom" {
		t.Fatalf("zh-TW open label %q", got)
	}
}

func TestTrayLanguageFallsBackWithoutProvider(t *testing.T) {
	want := i18n.First(osLocale())
	if got := (Menu{}).language(); got != want {
		t.Fatalf("Menu without Language = %q; want %q", got, want)
	}
	if got := (Menu{Language: func() i18n.Locale { return i18n.JA }}).language(); got != i18n.JA {
		t.Fatalf("Menu with Language = %q", got)
	}
}

func osLocale() i18n.Locale {
	l, _ := i18n.OS()
	return l
}
