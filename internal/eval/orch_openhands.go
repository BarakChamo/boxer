package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// OpenHands drives the OpenHands SDK through adapters/openhands: BoxerWorkspace routes every
// command through `boxer run`, so there is no harness CLI, no hook, and no rewrite to observe;
// the oracle checks that the command landed in the worktree's VM and not on the host. t1 calls
// the workspace directly with no model (the SDK's real LocalWorkspace base class, no fake needed);
// t2 runs a Conversation with a real Anthropic model and the SDK's terminal tool.
type OpenHands struct{}

func (OpenHands) Name() string { return "openhands" }

// python is the interpreter of the OpenHands virtualenv: BOXER_OPENHANDS_PYTHON, else
// .venv-openhands beside the working directory or the binary.
func (OpenHands) python() string {
	if p := os.Getenv("BOXER_OPENHANDS_PYTHON"); p != "" {
		return p
	}
	for _, root := range repoRoots() {
		p, _ := filepath.Abs(filepath.Join(root, ".venv-openhands", "bin", "python"))
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// repoRoots lists where the repository's non-Go files (adapters/, evals/) may live.
func repoRoots() []string {
	roots := []string{"."}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Join(filepath.Dir(exe), ".."))
	}
	return roots
}

func repoFile(rel string) string {
	for _, root := range repoRoots() {
		if p, err := filepath.Abs(filepath.Join(root, rel)); err == nil {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func (d OpenHands) Available(tier string) (bool, string) {
	if d.python() == "" {
		return false, "no OpenHands virtualenv: python3 -m venv .venv-openhands && .venv-openhands/bin/pip install openhands-sdk openhands-tools (or set BOXER_OPENHANDS_PYTHON)"
	}
	if repoFile("adapters/openhands/eval_run.py") == "" {
		return false, "adapters/openhands/eval_run.py not found beside the binary"
	}
	if tier == "t2" {
		if _, why := gatewayKey(); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (OpenHands) Cells(tier string) []Cell {
	// bash in the guest: OpenHands' terminal needs it, alpine has none.
	return []Cell{{Harness: "openhands", Mode: "rewrite", Entry: "sdk", Isolation: "worktree", Compliant: true, Tier: tier, Image: "mirror.gcr.io/library/node:24-bookworm-slim"}}
}

// Prepare writes boxer-bash for the terminal tool's shell_path.
func (d OpenHands) Prepare(env *Env, c Cell) error {
	out, err := env.boxer(env.Repo, "shim", "install", "--shell", d.shims(env))
	if err != nil {
		return fmt.Errorf("boxer shim install --shell: %v\n%s", err, out)
	}
	return nil
}

func (OpenHands) shims(env *Env) string { return filepath.Join(env.Work, "shims") }

func (d OpenHands) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	args := []string{repoFile("adapters/openhands/eval_run.py"), "--repo", env.Repo, "--command", env.Command()}
	if env.Tier == "t2" {
		args = append(args, "--live", "--prompt", prompt, "--model", "openai/"+LiveModel("openhands"), "--base-url", gatewayOpenAI,
			"--shell", filepath.Join(d.shims(env), "boxer-bash"))
	}
	cmd := exec.Command(d.python(), args...)
	cmd.Dir = env.Repo
	cmd.Env = append(env.BaseEnv(), "OPENHANDS_SUPPRESS_BANNER=1")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return Transcript{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(6 * time.Minute):
		cmd.Process.Kill()
		return Transcript{Raw: out.String()}, fmt.Errorf("openhands timed out")
	}
	tr := Transcript{Raw: out.String()}
	// The runner prints one JSON line {"answer": ..., "tools": [...], "skip": ...}; the SDK's
	// logger may print after it, so take the last line that parses.
	var res struct {
		Answer string   `json:"answer"`
		Tools  []string `json:"tools"`
		Skip   string   `json:"skip"`
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	found := false
	for i := len(lines) - 1; i >= 0 && !found; i-- {
		l := strings.TrimSpace(lines[i])
		found = strings.HasPrefix(l, `{"answer"`) && json.Unmarshal([]byte(l), &res) == nil
	}
	if !found {
		return tr, fmt.Errorf("openhands runner did not report a result line")
	}
	if res.Skip != "" {
		return tr, SkipError{res.Skip}
	}
	if f := strings.Fields(res.Answer); len(f) > 0 {
		tr.Answer = f[0]
	}
	tr.Tools = res.Tools
	return tr, nil
}

func (OpenHands) Cleanup(env *Env, c Cell) {}
