package codexcli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunnerUsesIsolatedReadOnlyCodexExec(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	fixture := t.TempDir()
	sourceHome := filepath.Join(fixture, "source")
	if err := os.Mkdir(sourceHome, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceHome, "auth.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(fixture, "codex")
	script := `#!/bin/sh
set -eu
test "$1" = "exec"
case " $* " in *" --sandbox read-only "*) ;; *) exit 21;; esac
case " $* " in *" --ephemeral "*) ;; *) exit 22;; esac
test -f "$CODEX_HOME/auth.json"
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output-last-message" ]; then shift; output="$1"; fi
  shift
done
prompt=$(cat)
printf 'result:%s' "$prompt" > "$output"
`
	if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	got, err := (Runner{Executable: fake, SourceHome: sourceHome, Timeout: time.Second}).Run(context.Background(), "analyze", "gpt-5.6-sol", "medium")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "result:analyze") {
		t.Fatalf("result = %q", got)
	}
}

func TestRunnerIntegration(t *testing.T) {
	if os.Getenv("CODEX_INTEGRATION") != "1" {
		t.Skip("set CODEX_INTEGRATION=1 to call the installed Codex CLI")
	}
	got, err := (Runner{Timeout: 10 * time.Minute}).Run(context.Background(), "Reply only OK", "gpt-5.6-sol", "medium")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != "OK" {
		t.Fatalf("result = %q, want OK", got)
	}
}
