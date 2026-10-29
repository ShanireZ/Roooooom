package identity

import (
	"context"
	"encoding/json"
	"html"
	"path/filepath"
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
	form := `<!doctype html><html lang="zh-Hant"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; form-action ` + html.EscapeString(s.origin) + `; base-uri 'none'"><title>Roooooom · 開始使用</title><style>body{margin:0;background:#f5f7fa;color:#253044;font:16px/1.7 system-ui,'Microsoft JhengHei',sans-serif}main{max-width:520px;margin:14vh auto;padding:36px;background:white;border:1px solid #e3e7ef;border-radius:14px}h1{font-size:27px}p{color:#6b7789}button{font:inherit;background:#3057d5;color:white;border:0;border-radius:9px;padding:11px 22px;cursor:pointer}small{display:block;color:#8b94a3;margin-top:24px}@media(max-width:600px){main{margin:8vh 16px;padding:26px}}</style><main><strong>Roooooom</strong><h1>本機協作已準備好</h1><p>確認你的管理身分，接著加入專案。各 agent 繼續在原本的宿主工作。</p><form method="post" action="` + html.EscapeString(s.origin) + `/api/v1/setup/activate"><input type="hidden" name="capability" value="` + cap + `"><button type="submit">開始使用</button></form><small>只在這台電腦啟用，這個入口五分鐘內有效。</small></main></html>`
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
