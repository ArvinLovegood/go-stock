package codexcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultTimeout = 10 * time.Minute

var runMu sync.Mutex

type Runner struct {
	Executable string
	SourceHome string
	Timeout    time.Duration
}

func (r Runner) Run(ctx context.Context, prompt, model, effort string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("prompt is empty")
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-5.6-sol"
	}
	switch effort {
	case "low", "medium", "high", "xhigh":
	default:
		effort = "medium"
	}
	if r.Executable == "" {
		r.Executable = "codex"
	}
	if r.Timeout <= 0 {
		r.Timeout = defaultTimeout
	}
	if r.SourceHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		r.SourceHome = filepath.Join(home, ".codex")
	}

	// The local CLI is intentionally serialized: deep analysis is expensive and
	// must never compete with scheduled direct-API jobs.
	runMu.Lock()
	defer runMu.Unlock()

	runHome, err := os.MkdirTemp("", "go-stock-codex-")
	if err != nil {
		return "", fmt.Errorf("create isolated CODEX_HOME: %w", err)
	}
	defer os.RemoveAll(runHome)
	for _, name := range []string{"auth.json", "config.toml"} {
		src := filepath.Join(r.SourceHome, name)
		if _, statErr := os.Stat(src); statErr == nil {
			if err := copyFile(src, filepath.Join(runHome, name), 0600); err != nil {
				return "", fmt.Errorf("copy Codex %s: %w", name, err)
			}
		}
	}

	outputPath := filepath.Join(runHome, "last-message.txt")
	runCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, r.Executable,
		"exec", "--ephemeral", "--sandbox", "read-only", "--skip-git-repo-check",
		"--model", model, "-c", fmt.Sprintf("model_reasoning_effort=%q", effort),
		"--output-last-message", outputPath, "-")
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = isolatedEnv(runHome)
	combined, err := cmd.CombinedOutput()
	if runCtx.Err() != nil {
		return "", fmt.Errorf("Codex deep analysis timed out after %s", r.Timeout)
	}
	if err != nil {
		return "", fmt.Errorf("codex exec failed: %w: %s", err, truncate(string(combined), 1200))
	}
	answer, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("read Codex result: %w", err)
	}
	if strings.TrimSpace(string(answer)) == "" {
		return "", errors.New("codex exec returned an empty result")
	}
	return strings.TrimSpace(string(answer)), nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func isolatedEnv(codexHome string) []string {
	allowed := []string{"PATH", "TMPDIR", "LANG", "LC_ALL", "SSL_CERT_FILE", "SSL_CERT_DIR"}
	env := []string{"CODEX_HOME=" + codexHome, "HOME=" + filepath.Dir(codexHome)}
	for _, key := range allowed {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
