package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/i18n"
	"roooooom.local/roooooom/internal/identity"
	"roooooom.local/roooooom/internal/store"
)

func expiredPage(t *testing.T, acceptLanguage string) *httptest.ResponseRecorder {
	t.Helper()
	ctx := context.Background()
	st, e := store.Open(ctx, filepath.Join(t.TempDir(), "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	ids := identity.New(st, nil)
	handler := httpapi.New(coordinator.New(st, nil, nil), ids, httpapi.Options{AllowedHost: "127.0.0.1:7337", BodyLimitBytes: 1024})
	path, e := ids.IssueOwnerBootstrap(ctx)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(path)
	capability := regexp.MustCompile(`name="capability" value="([^"]+)"`).FindSubmatch(raw)
	if len(capability) != 2 {
		t.Fatal("missing capability")
	}
	post := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7337/api/v1/setup/activate", strings.NewReader(url.Values{"capability": {string(capability[1])}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "null")
		r.Header.Set("Accept", "text/html")
		if acceptLanguage != "" {
			r.Header.Set("Accept-Language", acceptLanguage)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := post(); w.Code != http.StatusSeeOther {
		t.Fatal("first activation", w.Code, w.Body.String())
	}
	return post()
}

func TestExpiredPageAcceptLanguage(t *testing.T) {
	w := expiredPage(t, "fr, ja;q=0.8")
	body := w.Body.String()
	if w.Code != 409 || !strings.Contains(body, `<html lang="ja">`) || !strings.Contains(body, i18n.T(i18n.JA, i18n.ExpiredHeading)) {
		t.Fatalf("expired page not in Japanese: %d\n%s", w.Code, body)
	}
	if strings.Contains(body, "<style") || !strings.Contains(body, `<link rel="stylesheet" href="/assets/server-pages.css">`) {
		t.Fatalf("expired page must use the same-origin stylesheet:\n%s", body)
	}
}

func TestExpiredPageFallsBackToEnglish(t *testing.T) {
	w := expiredPage(t, "")
	if body := w.Body.String(); !strings.Contains(body, `<html lang="en">`) || !strings.Contains(body, i18n.T(i18n.EN, i18n.ExpiredHeading)) {
		t.Fatalf("expired page without a language signal is not English:\n%s", body)
	}
}
