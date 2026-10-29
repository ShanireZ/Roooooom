package launcher_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/control"
	"roooooom.local/roooooom/internal/launcher"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	appruntime "roooooom.local/roooooom/internal/runtime"
	"sync/atomic"
	"testing"
)

type fakePlatform struct {
	starts, opens, trays int
	stop                 bool
	noTray               bool
}

func (p *fakePlatform) StartOwnService(context.Context, string, []string) error {
	p.starts++
	return nil
}
func (p *fakePlatform) OpenOwnUI(string) error { p.opens++; return nil }
func (p *fakePlatform) ShowTray(m launcher.Menu) error {
	p.trays++
	if p.noTray {
		return launcher.ErrTrayUnavailable
	}
	if p.stop {
		return m.Stop()
	}
	return nil
}
func fixture(t *testing.T, foreign bool) (launcher.Config, *atomic.Int32) {
	t.Helper()
	root := t.TempDir()
	bundle := t.TempDir()
	core := filepath.Join(bundle, launcher.CoreName())
	os.WriteFile(core, []byte("fixture-not-executed"), 0700)
	stops := &atomic.Int32{}
	rt, e := appruntime.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { rt.Close() })
	var controller *control.Server
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity" {
			id := "s"
			if foreign {
				id = "foreign"
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(model.JSON(map[string]any{"status": "ok", "protocolVersion": 1, "serviceId": id, "datasetGeneration": "g"}))
			return
		}
		controller.ServeHTTP(w, r)
	}))
	address := server.Listener.Addr().String()
	controller, e = control.New(control.Config{Root: root, Address: address, Executable: core, ServiceID: "s", Generation: "g", Stop: func() { stops.Add(1); rt.Close() }, NeedsActivation: func(context.Context) (bool, error) { return false, nil }})
	if e != nil {
		t.Fatal(e)
	}
	if e = privatefs.WriteAtomic(root, "discovery.json", model.JSON(map[string]any{"listenAddr": address, "serviceId": "s", "datasetGeneration": "g", "protocolVersion": 1})); e != nil {
		t.Fatal(e)
	}
	server.Start()
	t.Cleanup(server.Close)
	return launcher.Config{Root: root, BundleDir: bundle, NoOpen: true}, stops
}
func TestLauncherRejectsForeignService(t *testing.T) {
	cfg, stops := fixture(t, true)
	p := &fakePlatform{stop: true}
	if e := launcher.Run(t.Context(), cfg, p); e == nil {
		t.Fatal("foreign service accepted")
	}
	if stops.Load() != 0 || p.starts != 0 || p.opens != 0 || p.trays != 0 {
		t.Fatal("foreign control", p, stops.Load())
	}
}
func TestTrayExitVsStop(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(string(model.JSON(stop)), func(t *testing.T) {
			cfg, stops := fixture(t, false)
			p := &fakePlatform{stop: stop}
			if e := launcher.Run(t.Context(), cfg, p); e != nil {
				t.Fatal(e)
			}
			want := int32(0)
			if stop {
				want = 1
			}
			if stops.Load() != want || p.starts != 0 {
				t.Fatal(stops.Load(), p.starts)
			}
		})
	}
}
func TestNoTrayStillRunsCore(t *testing.T) {
	cfg, stops := fixture(t, false)
	p := &fakePlatform{noTray: true}
	if e := launcher.Run(t.Context(), cfg, p); e != nil {
		t.Fatal(e)
	}
	if stops.Load() != 0 {
		t.Fatal("headless fallback stopped service")
	}
}
func TestLauncherRejectsForeignOpenTarget(t *testing.T) {
	cfg, _ := fixture(t, false)
	cfg.BundleDir = filepath.Join(cfg.BundleDir, "..")
	p := &fakePlatform{}
	if e := launcher.Run(t.Context(), cfg, p); e == nil {
		t.Fatal("foreign binary accepted")
	}
}
