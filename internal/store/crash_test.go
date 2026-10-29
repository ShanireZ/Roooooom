package store

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCommitCrashBoundary(t *testing.T) {
	if phase := os.Getenv("ROOOOOOM_TEST_CRASH"); phase != "" {
		s, err := Open(context.Background(), os.Getenv("ROOOOOOM_TEST_DB"))
		if err != nil {
			t.Fatal(err)
		}
		if phase == "before" {
			s.beforeCommit = func() { os.Exit(31) }
		} else {
			s.afterCommit = func() { os.Exit(31) }
		}
		_, _ = mutate(s, "hash")
		os.Exit(32)
	}
	for _, phase := range []string{"before", "after"} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.sqlite")
			s, err := Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			s.Close()
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(exe, "-test.run=^TestCommitCrashBoundary$")
			hideCrash(cmd)
			cmd.Env = append(os.Environ(), "ROOOOOOM_TEST_CRASH="+phase, "ROOOOOOM_TEST_DB="+path)
			output, err := cmd.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 31 {
				t.Fatalf("wrong crash: %v %s", err, output)
			}
			s, err = Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			var count int
			if err = s.reader.QueryRow("SELECT count(*) FROM events").Scan(&count); err != nil {
				t.Fatal(err)
			}
			want := 0
			if phase == "after" {
				want = 1
			}
			if count != want {
				t.Fatal("crash boundary", count, want)
			}
			if _, err = mutate(s, "hash"); err != nil {
				t.Fatal(err)
			}
			if err = s.reader.QueryRow("SELECT count(*) FROM events").Scan(&count); err != nil || count != 1 {
				t.Fatal("retry lost or duplicated")
			}
		})
	}
}
