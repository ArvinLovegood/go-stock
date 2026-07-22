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

func TestRunnerCanBeCancelled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	fixture := t.TempDir()
	fake := filepath.Join(fixture, "codex")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexec sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err := (Runner{Executable: fake, SourceHome: fixture, Timeout: time.Minute}).Run(ctx, "analyze", "gpt-5.6-sol", "medium")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "cancel") {
		t.Fatalf("error = %v, want cancellation", err)
	}
}

func TestRunnerQueueWaitCountsTowardTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	fixture := t.TempDir()
	fake := filepath.Join(fixture, "codex")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexec sleep 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = (Runner{Executable: fake, SourceHome: fixture, Timeout: 2 * time.Second}).Run(context.Background(), "first", "gpt-5.6-sol", "medium")
	}()
	time.Sleep(75 * time.Millisecond)
	started := time.Now()
	_, err := (Runner{Executable: fake, SourceHome: fixture, Timeout: 75 * time.Millisecond}).Run(context.Background(), "second", "gpt-5.6-sol", "medium")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "timed out") {
		t.Fatalf("error = %v, want queue timeout", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("queue timeout took %s", elapsed)
	}
	<-firstDone
}
