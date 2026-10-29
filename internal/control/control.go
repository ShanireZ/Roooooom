package control

import (
	"context"
	"crypto/subtle"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"strconv"
)

type File struct {
	ProtocolVersion int    `json:"protocolVersion"`
	ServiceID       string `json:"serviceId"`
	Generation      string `json:"datasetGeneration"`
	Address         string `json:"listenAddr"`
	Root            string `json:"dataRoot"`
	Executable      string `json:"executable"`
	Token           string `json:"token"`
}
type Proof struct {
	ProtocolVersion int    `json:"protocolVersion"`
	ServiceID       string `json:"serviceId"`
	Generation      string `json:"datasetGeneration"`
	Root            string `json:"dataRoot"`
	Executable      string `json:"executable"`
	NeedsActivation bool   `json:"needsActivation"`
}
type Config struct {
	Root, Address, Executable, ServiceID, Generation string
	Stop                                             func()
	Activate                                         func(context.Context) (string, error)
	NeedsActivation                                  func(context.Context) (bool, error)
}
type Server struct {
	file File
	cfg  Config
}

func ValidAddress(address string) bool {
	host, port, e := net.SplitHostPort(address)
	n, pe := strconv.Atoi(port)
	return e == nil && pe == nil && host == "127.0.0.1" && n > 0 && n <= 65535
}
func New(c Config) (*Server, error) {
	if !ValidAddress(c.Address) || c.ServiceID == "" || c.Generation == "" || !filepath.IsAbs(c.Root) || !filepath.IsAbs(c.Executable) {
		return nil, model.Invalid("invalid own control configuration")
	}
	f := File{1, c.ServiceID, c.Generation, c.Address, filepath.Clean(c.Root), filepath.Clean(c.Executable), string((model.RandomIDs{}).NewID())}
	if e := privatefs.WriteAtomic(c.Root, "control.json", model.JSON(f)); e != nil {
		return nil, e
	}
	return &Server{f, c}, nil
}
func Read(root string) (File, error) {
	var f File
	raw, e := privatefs.ReadFile(root, "control.json")
	if e != nil {
		return f, e
	}
	if e = model.Decode(raw, &f); e != nil {
		return f, e
	}
	if f.ProtocolVersion != 1 || !ValidAddress(f.Address) || len(f.Token) < 32 || len(f.Token) > 128 || !filepath.IsAbs(f.Root) || !filepath.IsAbs(f.Executable) {
		return f, model.Invalid("invalid own control file")
	}
	return f, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	reject := func(status int, code string) {
		w.WriteHeader(status)
		w.Write(model.JSON(map[string]any{"error": map[string]string{"code": code}}))
	}
	expected := "Bearer " + s.file.Token
	if r.Host != s.file.Address || r.Method != "POST" || r.URL.RawQuery != "" || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+s.file.Address) || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
		reject(403, "control-forbidden")
		return
	}
	if r.Header.Get("Content-Type") != "application/json" || r.ContentLength > 1024 {
		reject(400, "invalid-control-request")
		return
	}
	body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
	if e != nil {
		reject(400, "invalid-control-request")
		return
	}
	var p struct {
		ServiceID  string `json:"serviceId"`
		Generation string `json:"datasetGeneration"`
	}
	if model.Decode(body, &p) != nil {
		reject(400, "invalid-control-request")
		return
	}
	if p.ServiceID != s.file.ServiceID || p.Generation != s.file.Generation {
		reject(409, "control-identity-changed")
		return
	}
	switch r.URL.Path {
	case "/api/v1/control/probe":
		needs := true
		if s.cfg.NeedsActivation != nil {
			needs, e = s.cfg.NeedsActivation(r.Context())
			if e != nil {
				reject(503, "owner-state-unavailable")
				return
			}
		}
		w.Write(model.JSON(Proof{1, s.file.ServiceID, s.file.Generation, s.file.Root, s.file.Executable, needs}))
	case "/api/v1/control/activate":
		if s.cfg.Activate == nil {
			reject(503, "activation-unavailable")
			return
		}
		path, e := s.cfg.Activate(r.Context())
		if e != nil {
			reject(503, "activation-unavailable")
			return
		}
		w.Write(model.JSON(map[string]string{"activationPath": path}))
	case "/api/v1/control/stop":
		if s.cfg.Stop == nil {
			reject(503, "stop-unavailable")
			return
		}
		w.WriteHeader(202)
		w.Write([]byte(`{"stopping":true}`))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		go s.cfg.Stop()
	default:
		reject(404, "unknown-control-action")
	}
}
