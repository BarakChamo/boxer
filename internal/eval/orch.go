package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// checklist is an orchestrator that cannot be driven headlessly on this machine yet. It reports
// one skipped cell naming what is needed, so the report never presents its absence as a pass;
// the manual procedure is in docs/orchestrators.md.
type checklist struct {
	name string
	why  func() string
}

func (c checklist) Name() string { return c.name }
func (c checklist) Available(tier string) (bool, string) {
	return false, c.why() + "; checklist in docs/orchestrators.md"
}
func (c checklist) Cells(tier string) []Cell {
	return []Cell{{Harness: c.name, Mode: "rewrite", Entry: "project", Isolation: "worktree", Compliant: true, Tier: tier}}
}
func (checklist) Prepare(*Env, Cell) error                   { return nil }
func (checklist) Run(*Env, Cell, string) (Transcript, error) { return Transcript{}, nil }
func (checklist) Cleanup(*Env, Cell)                         {}

func installed(bin string) bool { _, err := exec.LookPath(bin); return err == nil }

// Multica: the local daemon runs the CLI in a worktree per task, but only against a configured
// Multica server (`multica setup`, an account); MULTICA_CLAUDE_ARGS carries extra CLI flags (spike 9).
var Multica = checklist{"multica", func() string {
	if !installed("multica") {
		return "multica is not installed (brew install multica-ai/tap/multica)"
	}
	if out, err := exec.Command("multica", "auth", "status").CombinedOutput(); err != nil || len(out) == 0 || strings.Contains(string(out), "No server configured") {
		return "multica has no server configured (multica setup, then multica daemon start)"
	}
	return "multica driver not automated: run the checklist"
}}

// server is an orchestrator process the driver started: output is captured for the report and
// the whole process group is killed on stop (Paperclip forks an embedded database, t3 a web app).
type server struct {
	cmd *exec.Cmd
	mu  sync.Mutex
	out bytes.Buffer
}

func startServer(dir string, env []string, name string, args ...string) (*server, error) {
	s := &server{cmd: exec.Command(name, args...)}
	s.cmd.Dir = dir
	s.cmd.Env = env
	s.cmd.Stdin = strings.NewReader("")
	s.cmd.Stdout, s.cmd.Stderr = &lockedWriter{s}, &lockedWriter{s}
	s.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return s, s.cmd.Start()
}

type lockedWriter struct{ s *server }

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	return w.s.out.Write(p)
}

func (s *server) output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.String()
}

// waitFor blocks until the output matches re (returning the first submatch) or the deadline passes.
func (s *server) waitFor(re *regexp.Regexp, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m := re.FindStringSubmatch(s.output()); m != nil {
			return m[len(m)-1], nil
		}
		if s.cmd.ProcessState != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", fmt.Errorf("%s did not print %s within %s", s.cmd.Path, re, timeout)
}

func (s *server) stop() {
	if s == nil || s.cmd.Process == nil {
		return
	}
	syscall.Kill(-s.cmd.Process.Pid, syscall.SIGTERM)
	done := make(chan struct{})
	go func() { s.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
	}
}

// freePort asks the kernel for an unused loopback port.
func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// httpJSON sends body (marshalled when non-nil) and decodes the reply into out (when non-nil).
func httpJSON(method, url string, headers map[string]string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s: %s", method, url, resp.Status, strings.TrimSpace(string(b)))
	}
	if out != nil && len(b) > 0 {
		return json.Unmarshal(b, out)
	}
	return nil
}

// lastKernel is the last kernel name in a transcript: a live model phrases its answer, and the
// orchestrators wrap it in their own run logs.
func lastKernel(raw string) string {
	if m := reKernel.FindAllString(raw, -1); len(m) > 0 {
		return m[len(m)-1]
	}
	return ""
}
