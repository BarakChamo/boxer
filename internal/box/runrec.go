package box

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/sh"
)

// RunRecord is what the last command in a scope did. It exists so a failure can be explained after
// the fact and turned into a capsule, and for nothing else: no decision anywhere reads one, so
// deleting the directory costs a forgotten last run and never a broken sandbox.
type RunRecord struct {
	Time     time.Time `json:"time"`
	Scope    string    `json:"scope"`
	Task     string    `json:"task,omitempty"`
	Command  string    `json:"command"`
	Dir      string    `json:"dir"` // worktree-relative
	Exit     int       `json:"exit"`
	Duration float64   `json:"duration_ms"`
	Image    string    `json:"image,omitempty"`
	Pack     string    `json:"pack,omitempty"`
	GitHead  string    `json:"git_head,omitempty"`
	Dirty    bool      `json:"dirty"`
	Config   []string  `json:"config_files,omitempty"`
	Stdout   string    `json:"stdout_tail,omitempty"`
	Stderr   string    `json:"stderr_tail,omitempty"`
	// Tests is the parsed JUnit summary, attached by the caller that knows where the XML is.
	Tests *TestSummary `json:"tests,omitempty"`
}

// TestSummary is the part of a JUnit parse worth keeping in a record. It duplicates junit.Summary
// rather than importing it, because internal/junit imports nothing and should keep it that way.
type TestSummary struct {
	Tests    int      `json:"tests"`
	Failures int      `json:"failures"`
	Errors   int      `json:"errors"`
	Skipped  int      `json:"skipped"`
	Failed   []string `json:"failed,omitempty"`
}

// RunsDir holds one record per scope, named by scope key.
func RunsDir() string {
	return filepath.Join(filepath.Dir(LastUsedDir()), "runs")
}

// RunRecordPath is where a scope's last run is written.
func RunRecordPath(key string) string { return filepath.Join(RunsDir(), key+".json") }

// WriteRunRecord stores r, replacing whatever was there. Every failure is ignored: a record is a
// convenience and must never turn a successful command into a failed one.
func WriteRunRecord(r RunRecord) {
	if err := os.MkdirAll(RunsDir(), 0o755); err != nil {
		return
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return
	}
	// 0600: the record holds the command line verbatim, which is the one place boxer keeps
	// something a redacting telemetry sink would have stripped.
	_ = writeAtomic(RunRecordPath(r.Scope), b, 0o600)
}

// writeAtomic replaces path with b in one step. Parallel runs on a scope (an agent's parallel tool
// calls) each truncated and wrote the same file, and could leave one's bytes over the other's
// tail. A temporary file left by a killed process carries the scope's name, so the sweep finds it.
func writeAtomic(path string, b []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(f.Name(), perm)
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		_ = os.Remove(f.Name())
	}
	return err
}

// ReadRunRecord returns the scope's last run, if there is one.
func ReadRunRecord(key string) (RunRecord, bool) {
	b, err := os.ReadFile(RunRecordPath(key))
	if err != nil {
		return RunRecord{}, false
	}
	var r RunRecord
	if json.Unmarshal(b, &r) != nil {
		return RunRecord{}, false
	}
	return r, true
}

// AmendRunTests attaches a test summary to the record already written for this scope. The caller
// that parses JUnit knows the worktree-relative globs; Run, which wrote the record, does not.
func AmendRunTests(key string, s *TestSummary) {
	r, ok := ReadRunRecord(key)
	if !ok {
		return
	}
	r.Tests = s
	WriteRunRecord(r)
}

// tail keeps the last n bytes written through it, so a record can quote the end of a failure
// without buffering the whole stream and without delaying a single byte of it.
//
// ponytail: byte ring, not line-aware — a multi-byte rune can be cut at the boundary.
type tail struct {
	b []byte
	n int
}

func (t *tail) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > t.n {
		t.b = t.b[len(t.b)-t.n:]
	}
	return len(p), nil
}

func (t *tail) String() string { return string(t.b) }

// CommandLine renders argv as one shell line that means the same thing when run again. A record
// that stored the joined argv of `sh -c "exit 4"` would replay as `sh -c sh -c exit 4`, which is
// a different command with a different exit code.
func CommandLine(argv []string) string {
	if len(argv) == 3 && argv[0] == "sh" && argv[1] == "-c" {
		return argv[2]
	}
	parts := make([]string, 0, len(argv))
	for _, a := range argv {
		if a == "" || strings.ContainsAny(a, " \t\n'\"\\$`&|;<>()*?[]{}#~!") {
			a = sh.Quote(a)
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

// gitState reports the worktree's HEAD and whether it has uncommitted changes, so a replay can say
// what it is replaying against. git is a host passthrough everywhere else in boxer too.
func gitState(root string) (head string, dirty bool) {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", false
	}
	head = strings.TrimSpace(string(out))
	st, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	return head, err == nil && len(strings.TrimSpace(string(st))) > 0
}

// gitInfo carries gitState's answer from the goroutine that collects it during the run.
type gitInfo struct {
	head  string
	dirty bool
}
