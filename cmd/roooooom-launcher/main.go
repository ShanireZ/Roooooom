package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"roooooom.local/roooooom/internal/buildinfo"
	"roooooom.local/roooooom/internal/launcher"
	appruntime "roooooom.local/roooooom/internal/runtime"
)

func main() {
	root := flag.String("data-dir", "", "product private data directory")
	version := flag.Bool("version", false, "print embedded build metadata without starting a service")
	listen := flag.String("listen", "127.0.0.1:7337", "loopback address")
	headless := flag.Bool("headless", false, "keep core running without opening browser or tray")
	noOpen := flag.Bool("no-open", false, "show own tray without automatically opening UI")
	stop := flag.Bool("stop", false, "explicitly stop only this product")
	activate := flag.Bool("activate", false, "renew private owner activation page")
	open := flag.Bool("open", false, "open own UI without a second tray")
	flag.Parse()
	if *version {
		if flag.NArg() != 0 {
			os.Exit(2)
		}
		_ = json.NewEncoder(os.Stdout).Encode(buildinfo.Current())
		return
	}
	if flag.NArg() != 0 {
		os.Exit(2)
	}
	count := 0
	for _, b := range []bool{*stop, *activate, *open} {
		if b {
			count++
		}
	}
	if count > 1 {
		os.Exit(2)
	}
	if *root == "" {
		var e error
		*root, e = appruntime.DefaultRoot()
		if e != nil {
			fmt.Fprintln(os.Stderr, "product private root unavailable")
			os.Exit(1)
		}
	}
	absolute, e := filepath.Abs(*root)
	if e != nil {
		os.Exit(2)
	}
	exe, e := os.Executable()
	if e != nil {
		os.Exit(1)
	}
	mode := "run"
	if *stop {
		mode = "stop"
	}
	if *activate {
		mode = "activate"
	}
	if *open {
		mode = "open"
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	e = launcher.Run(ctx, launcher.Config{Root: absolute, BundleDir: filepath.Dir(exe), Listen: *listen, Mode: mode, NoOpen: *headless || *noOpen, NoTray: *headless}, launcher.Native{})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		if !*headless {
			launcher.ShowError(e)
		}
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "Roooooom operation completed.")
}
