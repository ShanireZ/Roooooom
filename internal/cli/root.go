package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/buildinfo"
	"roooooom.local/roooooom/internal/control"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/identity"
	"roooooom.local/roooooom/internal/outbound"
	appruntime "roooooom.local/roooooom/internal/runtime"
	"roooooom.local/roooooom/internal/store"
	"time"
)

func Run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) > 0 && args[0] == "version" {
		if len(args) > 2 || len(args) == 2 && args[1] != "--json" {
			return 2
		}
		if json.NewEncoder(out).Encode(buildinfo.Current()) != nil {
			return 1
		}
		return 0
	}
	if len(args) > 0 && args[0] == "admin" {
		return runAdmin(ctx, args[1:], out, errOut)
	}
	if len(args) == 0 || args[0] != "serve" {
		return runClient(ctx, args, in, out, errOut)
	}
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(errOut)
	dataDir := flags.String("data-dir", "", "private data directory")
	listen := flags.String("listen", "127.0.0.1:7337", "loopback address")
	dbLimit := flags.Int64("database-limit-bytes", appruntime.DefaultDatabaseLimit, "SQLite hard limit in bytes; warning at 75%")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return 2
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || host != "127.0.0.1" {
		fmt.Fprintln(errOut, "only 127.0.0.1 is supported")
		return 2
	}
	if *dataDir == "" {
		*dataDir, err = appruntime.DefaultRoot()
		if err != nil {
			fmt.Fprintln(errOut, "private data directory unavailable")
			return 1
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return 1
	}
	absRoot, err := filepath.Abs(*dataDir)
	if err != nil {
		return 2
	}
	if appruntime.InstallationContainsDataRoot(filepath.Dir(executable), absRoot) {
		fmt.Fprintln(errOut, "data directory must be outside the installation")
		return 2
	}
	*dataDir = absRoot
	rt, err := appruntime.Open(*dataDir)
	if err != nil {
		if errors.Is(err, appruntime.ErrAlreadyRunning) {
			fmt.Fprintln(errOut, err)
		} else {
			fmt.Fprintln(errOut, "cannot open private data directory")
		}
		return 1
	}
	defer rt.Close()
	db, err := store.Open(ctx, filepath.Join(rt.Root, "state.sqlite"))
	if err != nil {
		fmt.Fprintln(errOut, "cannot open coordination database")
		return 1
	}
	defer db.Close()
	queue := outbound.NewQueue(db, map[string]outbound.Adapter{"github": outbound.NewGitHub(outbound.ResolveCredential(rt.Root)), "dingtalk": outbound.NewDingTalk(outbound.ResolveCredential(rt.Root))})
	if err = queue.Recover(ctx); err != nil {
		fmt.Fprintln(errOut, "cannot recover outbound state")
		return 1
	}
	if err = appruntime.SetDatabaseLimit(ctx, db, *dbLimit); err != nil {
		fmt.Fprintln(errOut, "cannot apply database limit")
		return 1
	}
	databaseIdentity := db.Identity()
	rt.ServiceID, rt.Generation = databaseIdentity.ServiceID, databaseIdentity.Generation
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(errOut, "listen address unavailable")
		return 1
	}
	defer ln.Close()
	if err = rt.Publish(ln.Addr().String()); err != nil {
		fmt.Fprintln(errOut, "cannot publish service identity")
		return 1
	}
	identities := identity.New(db, nil)
	identities.SetOrigin("http://" + ln.Addr().String())
	serviceCtx, serviceCancel := context.WithCancel(ctx)
	defer serviceCancel()
	controller, err := control.New(control.Config{Root: rt.Root, Address: ln.Addr().String(), Executable: executable, ServiceID: rt.ServiceID, Generation: rt.Generation, Stop: serviceCancel, Activate: identities.IssueOwnerBootstrap, NeedsActivation: identities.NeedsOwnerActivation, OwnerLanguage: db.OwnerLanguage})
	if err != nil {
		fmt.Fprintln(errOut, "cannot initialize private own-service control")
		return 1
	}
	handler := httpapi.New(coordinator.New(db, nil, nil), identities, httpapi.Options{AllowedHost: ln.Addr().String(), Control: controller})
	if _, err = identities.IssueOwnerBootstrap(ctx); err != nil {
		fmt.Fprintln(errOut, "cannot create private activation page")
		return 1
	}
	queueCtx, queueCancel := context.WithCancel(serviceCtx)
	defer queueCancel()
	queueDone := make(chan struct{})
	go func() { defer close(queueDone); queue.Run(queueCtx) }()
	defer func() { queueCancel(); <-queueDone }()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	stop := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		select {
		case <-serviceCtx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if server.Shutdown(shutdown) != nil {
				_ = server.Close()
			}
		case <-stop:
		}
	}()
	err = server.Serve(ln)
	close(stop)
	<-shutdownDone
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(errOut, "service stopped unexpectedly")
		return 1
	}
	return 0
}
