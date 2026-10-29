package identity

import (
	"context"
	"os"
	"strings"
	"testing"

	"roooooom.local/roooooom/internal/i18n"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

const activationCSP = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; form-action http://127.0.0.1:7337; base-uri 'none'">`

func activationPage(t *testing.T, s *Service) string {
	t.Helper()
	path, e := s.IssueOwnerBootstrap(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return string(raw)
}

func TestActivationPageUsesOwnerLanguage(t *testing.T) {
	s, st, _ := fixture(t)
	if _, e := st.Write(context.Background(), func(tx *store.Tx) (model.Receipt, error) {
		return model.Receipt{}, tx.SetOwnerLanguage("ja")
	}); e != nil {
		t.Fatal(e)
	}
	page := activationPage(t, s)
	if !strings.Contains(page, `<html lang="ja">`) || !strings.Contains(page, i18n.T(i18n.JA, i18n.ActivationButton)) {
		t.Fatalf("activation page not in owner language:\n%s", page)
	}
	if !strings.Contains(page, activationCSP) {
		t.Fatalf("activation CSP changed:\n%s", page)
	}
}

func TestActivationPageFallsBackToOSLanguage(t *testing.T) {
	s, _, _ := fixture(t)
	os, _ := i18n.OS()
	want := i18n.First(os)
	page := activationPage(t, s)
	if !strings.Contains(page, `<html lang="`+string(want)+`">`) || !strings.Contains(page, i18n.T(want, i18n.ActivationHeading)) {
		t.Fatalf("activation page is not in %s:\n%s", want, page)
	}
}

func TestServerPageStylesheetMatchesInlineCSS(t *testing.T) {
	raw, e := os.ReadFile("../../web/public/assets/server-pages.css")
	if e != nil {
		t.Fatal(e)
	}
	if strings.ReplaceAll(string(raw), "\r\n", "\n") != ServerPageCSS {
		t.Fatal("web/public/assets/server-pages.css differs from identity.ServerPageCSS")
	}
}
