package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gofrs/flock"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/control"
	"roooooom.local/roooooom/internal/i18n"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	appruntime "roooooom.local/roooooom/internal/runtime"
	"runtime"
	"strings"
	"time"
)

var ErrTrayUnavailable = errors.New("tray unavailable; own service continues")
var ErrForeign = errors.New("service identity or installation differs; no control was attempted")
var ErrOffline = errors.New("own service is not reachable")

type Config struct {
	Root, BundleDir, Listen, Mode string
	NoOpen, NoTray                bool
}

func CoreName() string {
	if runtime.GOOS == "windows" {
		return "roooooom.exe"
	}
	return "roooooom"
}
func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// A historical installation path is reusable only when its endpoint refuses
// connections and its writer lock is free. Never send control credentials to it.
func stoppedInstallation(root, bundle string) bool {
	f, e := control.Read(root)
	if e != nil || !samePath(f.Root, root) || samePath(f.Executable, filepath.Join(bundle, CoreName())) {
		return false
	}
	if _, e = Connect(root, filepath.Dir(f.Executable)); e != nil {
		return false
	}
	conn, e := net.DialTimeout("tcp", f.Address, 500*time.Millisecond)
	if e == nil {
		conn.Close()
		return false
	}
	if !connectionRefused(e) {
		return false
	}
	rt, e := appruntime.Open(root)
	if e != nil {
		return false
	}
	return rt.Close() == nil
}

type Client struct {
	root, core string
	file       control.File
	http       *http.Client
}

func Connect(root, bundle string) (*Client, error) {
	core := filepath.Join(bundle, CoreName())
	f, e := control.Read(root)
	if e != nil {
		return nil, e
	}
	if !samePath(f.Root, root) || !samePath(f.Executable, core) {
		return nil, ErrForeign
	}
	raw, e := privatefs.ReadFile(root, "discovery.json")
	if e != nil {
		return nil, e
	}
	var d struct {
		Address         string `json:"listenAddr"`
		ServiceID       string `json:"serviceId"`
		Generation      string `json:"datasetGeneration"`
		ProtocolVersion int    `json:"protocolVersion"`
	}
	if model.Decode(raw, &d) != nil || d.ProtocolVersion != 1 || d.ServiceID != f.ServiceID || d.Generation != f.Generation || d.Address != f.Address {
		return nil, ErrForeign
	}
	return &Client{root, core, f, &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) check(ctx context.Context) error {
	current, e := control.Read(c.root)
	if e != nil {
		return e
	}
	if current != c.file {
		return ErrForeign
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://"+c.file.Address+"/identity", nil)
	response, e := c.http.Do(req)
	if e != nil {
		return ErrOffline
	}
	defer response.Body.Close()
	var identity struct {
		ServiceID       string `json:"serviceId"`
		Generation      string `json:"datasetGeneration"`
		ProtocolVersion int    `json:"protocolVersion"`
		Status          string `json:"status"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&identity) != nil || identity.ServiceID != c.file.ServiceID || identity.Generation != c.file.Generation || identity.ProtocolVersion != 1 || identity.Status != "ok" {
		return ErrForeign
	}
	return nil
}
func (c *Client) call(ctx context.Context, action string, target any) error {
	if e := c.check(ctx); e != nil {
		return e
	}
	raw := model.JSON(map[string]string{"serviceId": c.file.ServiceID, "datasetGeneration": c.file.Generation})
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://"+c.file.Address+"/api/v1/control/"+action, bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+c.file.Token)
	req.Header.Set("Content-Type", "application/json")
	r, e := c.http.Do(req)
	if e != nil {
		return ErrOffline
	}
	defer r.Body.Close()
	if r.StatusCode != 200 && r.StatusCode != 202 {
		return ErrForeign
	}
	if json.NewDecoder(io.LimitReader(r.Body, 16384)).Decode(target) != nil {
		return model.Invalid("invalid own control response")
	}
	return nil
}
func (c *Client) Probe(ctx context.Context) (control.Proof, error) {
	var p control.Proof
	e := c.call(ctx, "probe", &p)
	if e == nil && (p.ProtocolVersion != 1 || p.ServiceID != c.file.ServiceID || p.Generation != c.file.Generation || !samePath(p.Root, c.root) || !samePath(p.Executable, c.core)) {
		e = ErrForeign
	}
	return p, e
}
func (c *Client) Activate(ctx context.Context) (string, error) {
	if _, e := c.Probe(ctx); e != nil {
		return "", e
	}
	var p struct {
		Path string `json:"activationPath"`
	}
	if e := c.call(ctx, "activate", &p); e != nil {
		return "", e
	}
	expected := filepath.Join(c.root, "activation.html")
	if !samePath(p.Path, expected) {
		return "", ErrForeign
	}
	return expected, nil
}
func (c *Client) Stop(ctx context.Context) error {
	if _, e := c.Probe(ctx); e != nil {
		return e
	}
	var p struct {
		Stopping bool `json:"stopping"`
	}
	if e := c.call(ctx, "stop", &p); e != nil {
		return e
	}
	if !p.Stopping {
		return ErrForeign
	}
	timer := time.NewTicker(50 * time.Millisecond)
	defer timer.Stop()
	deadline := time.NewTimer(8 * time.Second)
	defer deadline.Stop()
	for {
		rt, e := appruntime.Open(c.root)
		if e == nil {
			rt.Close()
			return nil
		}
		if !errors.Is(e, appruntime.ErrAlreadyRunning) {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("own service stop not confirmed")
		case <-timer.C:
		}
	}
}
func fileURL(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}
func (c *Client) open(ctx context.Context, p Platform, activate bool) error {
	proof, e := c.Probe(ctx)
	if e != nil {
		return e
	}
	target := "http://" + c.file.Address + "/"
	if activate || proof.NeedsActivation {
		path, e := c.Activate(ctx)
		if e != nil {
			return e
		}
		target = fileURL(path)
	}
	return p.OpenOwnUI(target)
}
func Run(ctx context.Context, cfg Config, p Platform) error {
	if !filepath.IsAbs(cfg.Root) || !filepath.IsAbs(cfg.BundleDir) {
		return model.Invalid("absolute product paths required")
	}
	if cfg.Mode != "" && cfg.Mode != "run" && cfg.Mode != "open" && cfg.Mode != "activate" && cfg.Mode != "stop" {
		return model.Invalid("unknown launcher mode")
	}
	if e := privatefs.CheckRoot(cfg.BundleDir); e != nil {
		return e
	}
	if cfg.Mode != "stop" && appruntime.InstallationContainsDataRoot(cfg.BundleDir, cfg.Root) {
		return model.Invalid("data directory must be outside the installation")
	}
	core := filepath.Join(cfg.BundleDir, CoreName())
	info, e := os.Lstat(core)
	if e != nil || !info.Mode().IsRegular() {
		return model.Invalid("bundled core binary missing")
	}
	if e = privatefs.EnsureDir(cfg.Root); e != nil {
		return e
	}
	if cfg.Mode == "stop" {
		client, e := Connect(cfg.Root, cfg.BundleDir)
		if e != nil {
			return e
		}
		return client.Stop(ctx)
	}
	lock := flock.New(filepath.Join(cfg.Root, "launcher.lock"))
	held, e := lock.TryLock()
	if e != nil {
		return e
	}
	defer lock.Close()
	client, e := Connect(cfg.Root, cfg.BundleDir)
	if e == nil {
		_, e = client.Probe(ctx)
	}
	if e != nil {
		if held && errors.Is(e, ErrForeign) && stoppedInstallation(cfg.Root, cfg.BundleDir) {
			e = ErrOffline
		}
		if !held {
			return e
		}
		if !errors.Is(e, os.ErrNotExist) && !errors.Is(e, ErrOffline) {
			return e
		}
		listen := cfg.Listen
		if listen == "" {
			listen = "127.0.0.1:7337"
		}
		if listen != "127.0.0.1:0" && !control.ValidAddress(listen) {
			return model.Invalid("loopback listen address required")
		}
		if e = p.StartOwnService(ctx, core, []string{"serve", "--data-dir", cfg.Root, "--listen", listen}); e != nil {
			return e
		}
		deadline := time.NewTimer(10 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
	ready:
		for {
			client, e = Connect(cfg.Root, cfg.BundleDir)
			if e == nil {
				_, e = client.Probe(ctx)
				if e == nil {
					break ready
				}
				if !errors.Is(e, ErrOffline) {
					return e
				}
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-deadline.C:
				return errors.New("own service did not become ready; check its port and data directory")
			case <-ticker.C:
			}
		}
	}
	if !cfg.NoOpen {
		if e = client.open(ctx, p, cfg.Mode == "activate"); e != nil {
			return e
		}
	}
	if cfg.NoOpen && cfg.Mode == "activate" {
		if _, e = client.Activate(ctx); e != nil {
			return e
		}
	}
	if !held || cfg.NoTray || cfg.Mode == "open" || cfg.Mode == "activate" {
		return nil
	}
	e = p.ShowTray(Menu{Open: func() error { return client.open(context.Background(), p, false) }, Stop: func() error { return client.Stop(context.Background()) }, Activate: func() error { return client.open(context.Background(), p, true) }, Language: func() i18n.Locale {
		proof, e := client.Probe(context.Background())
		if e != nil {
			return ""
		}
		l, _ := i18n.Parse(proof.OwnerLanguage)
		return l
	}})
	if errors.Is(e, ErrTrayUnavailable) {
		return nil
	}
	return e
}
