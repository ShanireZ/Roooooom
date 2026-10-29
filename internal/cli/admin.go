package cli

import (
	"context"
	"flag"
	"io"
	"path/filepath"
	"regexp"
	"roooooom.local/roooooom/internal/backup"
	"roooooom.local/roooooom/internal/model"
	appruntime "roooooom.local/roooooom/internal/runtime"
	"roooooom.local/roooooom/internal/store"
)

// These are explicit operator filesystem commands. Agent bearer credentials
// grant no access; portable skills never invoke admin commands.
func runAdmin(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) < 1 {
		return 2
	}
	flags := flag.NewFlagSet("admin", flag.ContinueOnError)
	flags.SetOutput(errOut)
	root := flags.String("data-dir", "", "explicit product data directory")
	snapshot := flags.String("snapshot", "", "product-owned snapshot ID")
	confirm := flags.String("confirm-service-id", "", "exact service ID being restored")
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 || *root == "" {
		return 2
	}
	if args[0] == "restore" {
		if !regexp.MustCompile("^[A-Za-z0-9_-]{1,128}$").MatchString(*snapshot) || *confirm == "" {
			return report(out, model.Invalid("snapshot ID and exact service confirmation required"))
		}
		target := filepath.Join(*root, "backups", *snapshot)
		manifest, e := backup.Validate(ctx, target)
		if e != nil {
			return report(out, e)
		}
		if manifest.ServiceID != *confirm {
			return report(out, model.Conflict("service-mismatch"))
		}
		if e = backup.Restore(ctx, *root, target); e != nil {
			return report(out, e)
		}
		return printJSON(out, map[string]any{"restored": true, "ownerReactivationRequired": true})
	}
	if args[0] != "backup" && args[0] != "export" {
		return 2
	}
	rt, e := appruntime.Open(*root)
	if e != nil {
		return report(out, e)
	}
	defer rt.Close()
	st, e := store.Open(ctx, filepath.Join(rt.Root, "state.sqlite"))
	if e != nil {
		return report(out, e)
	}
	defer st.Close()
	id := string((model.RandomIDs{}).NewID())
	folder := "backups"
	if args[0] == "export" {
		folder = "exports"
	}
	target := filepath.Join(rt.Root, folder, id)
	if args[0] == "backup" {
		_, e = backup.Create(ctx, st, target)
	} else {
		e = backup.Export(ctx, st, target)
	}
	if e != nil {
		return report(out, e)
	}
	return printJSON(out, map[string]any{"id": id, "path": target})
}
