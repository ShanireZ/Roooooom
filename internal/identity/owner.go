package identity

import (
	"context"
	"encoding/json"
	"html"
	"path/filepath"
	"roooooom.local/roooooom/internal/i18n"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
	"time"
)

func (s *Service) NeedsOwnerActivation(ctx context.Context) (bool, error) {
	needed := true
	e := s.store.Read(ctx, func(reader store.Reader) error {
		rows, e := reader.QueryContext(ctx, "SELECT body FROM tokens WHERE kind='human' AND revoked=0")
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var raw string
			if e = rows.Scan(&raw); e != nil {
				return e
			}
			var body tokenBody
			if json.Unmarshal([]byte(raw), &body) == nil && !body.ExpiresAt.IsZero() && s.clock.Now().Before(body.ExpiresAt) {
				needed = false
				return nil
			}
		}
		return rows.Err()
	})
	return needed, e
}

func (s *Service) IssueOwnerBootstrap(ctx context.Context) (string, error) {
	if s.origin == "" {
		return "", model.Invalid("service origin not set")
	}
	cap := random()
	_, e := s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		if _, e := tx.Exec("UPDATE tokens SET revoked=1 WHERE kind='bootstrap'"); e != nil {
			return model.Receipt{}, e
		}
		e := insertToken(tx, cap, "owner-bootstrap", "bootstrap", nil, nil, tokenBody{ExpiresAt: s.clock.Now().Add(5 * time.Minute)})
		return model.Receipt{}, e
	})
	if e != nil {
		return "", e
	}
	// No HTTP request precedes this local file, so the owner language and then
	// the OS UI language choose its text.
	owner, e := s.store.OwnerLanguage(ctx)
	if e != nil {
		return "", e
	}
	system, _ := i18n.OS()
	l := i18n.First(i18n.Locale(owner), system)
	t := func(k i18n.Key) string { return html.EscapeString(i18n.T(l, k)) }
	form := `<!doctype html><html lang="` + string(l) + `"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; form-action ` + html.EscapeString(s.origin) + `; base-uri 'none'"><title>` + t(i18n.ActivationTitle) + `</title><style>` + ServerPageCSS + `</style><main><strong class="mark">ROOOOOOM</strong><h1>` + t(i18n.ActivationHeading) + `</h1><p>` + t(i18n.ActivationBody) + `</p><form method="post" action="` + html.EscapeString(s.origin) + `/api/v1/setup/activate"><input type="hidden" name="capability" value="` + cap + `"><button type="submit">` + t(i18n.ActivationButton) + `</button></form><small>` + t(i18n.ActivationFootnote) + `</small></main></html>`
	if e = privatefs.WriteAtomic(s.Root(), "activation.html", []byte(form)); e != nil {
		return "", e
	}
	return filepath.Join(s.Root(), "activation.html"), nil
}
func (s *Service) Activate(ctx context.Context, cap string) (string, string, error) {
	token, csrf := random(), random()
	_, e := s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		var raw string
		var revoked int
		if e := tx.QueryRow("SELECT body,revoked FROM tokens WHERE hash=? AND kind='bootstrap'", hash(cap)).Scan(&raw, &revoked); e != nil {
			return model.Receipt{}, model.Conflict("activation-expired")
		}
		var b tokenBody
		if revoked != 0 || json.Unmarshal([]byte(raw), &b) != nil || !s.clock.Now().Before(b.ExpiresAt) {
			return model.Receipt{}, model.Conflict("activation-expired")
		}
		if _, e := tx.Exec("UPDATE tokens SET revoked=1 WHERE hash=?", hash(cap)); e != nil {
			return model.Receipt{}, e
		}
		if e := insertToken(tx, token, "owner", "human", nil, nil, tokenBody{CSRF: csrf, ExpiresAt: s.clock.Now().Add(24 * time.Hour)}); e != nil {
			return model.Receipt{}, e
		}
		if e := privatefs.WriteAtomic(s.Root(), "credentials/owner.json", []byte(marshal(map[string]string{"token": token, "csrf": csrf}))); e != nil {
			return model.Receipt{}, e
		}
		seq, e := tx.AppendEvent("owner", []byte(`{"kind":"owner.activated"}`))
		return model.Receipt{EventID: seq}, e
	})
	if e != nil {
		return "", "", e
	}
	return token, csrf, nil
}
