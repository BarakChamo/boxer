package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/junit"
	"github.com/BarakChamo/boxer/internal/scope"
)

// capsule is a failure worth keeping: what ran, against what tree, in what environment, and what
// outcome counts as having reproduced it. It is a file in the repository, not boxer state — the
// point of writing one is that it outlives the sandbox and can be handed to someone else.
//
// It deliberately does not preserve a machine. A prepared machine is a named pack; this is the
// recipe and the oracle.
type capsule struct {
	Version int       `toml:"version" json:"version"`
	Created time.Time `toml:"created" json:"created"`
	Scope   string    `toml:"scope" json:"scope"`
	Image   string    `toml:"image,omitempty" json:"image,omitempty"`
	Pack    string    `toml:"pack,omitempty" json:"pack,omitempty"`

	Run    capsuleRun    `toml:"run" json:"run"`
	Git    capsuleGit    `toml:"git" json:"git"`
	Config capsuleConfig `toml:"config" json:"config"`
	Expect capsuleExpect `toml:"expect" json:"expect"`
}

type capsuleRun struct {
	Task    string `toml:"task,omitempty" json:"task,omitempty"`
	Command string `toml:"command" json:"command"`
	Dir     string `toml:"dir" json:"dir"`
}

type capsuleGit struct {
	Head  string `toml:"head,omitempty" json:"head,omitempty"`
	Dirty bool   `toml:"dirty" json:"dirty"`
	Patch string `toml:"patch,omitempty" json:"patch,omitempty"`
}

// capsuleConfig is only the configuration that changes what a command does. Replaying overlays
// these onto the resolved config instead of re-reading boxer.toml: a capsule that picked up
// whatever the repository says today would reproduce a different run.
type capsuleConfig struct {
	Image       string            `toml:"image,omitempty" json:"image,omitempty"`
	ImageSetup  []string          `toml:"image_setup,omitempty" json:"image_setup,omitempty"`
	Setup       []string          `toml:"setup,omitempty" json:"setup,omitempty"`
	Env         map[string]string `toml:"env,omitempty" json:"env,omitempty"`
	Mounts      []string          `toml:"mounts,omitempty" json:"mounts,omitempty"`
	NetworkMode string            `toml:"network_mode,omitempty" json:"network_mode,omitempty"`
	AllowHosts  []string          `toml:"allow_hosts,omitempty" json:"allow_hosts,omitempty"`
}

type capsuleExpect struct {
	Exit         int      `toml:"exit" json:"exit"`
	FailingTests []string `toml:"failing_tests,omitempty" json:"failing_tests,omitempty"`
}

func capsuleCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: boxer capsule new [-o capsule.toml] | inspect <path> | replay <path>")
		return 2
	}
	switch args[0] {
	case "new":
		return capsuleNew(args[1:], stdout, stderr)
	case "inspect":
		return capsuleInspect(args[1:], stdout, stderr)
	case "replay":
		return capsuleReplay(args[1:], stdin, stdout, stderr)
	}
	fmt.Fprintf(stderr, "boxer capsule: unknown subcommand %q; want new, inspect or replay\n", args[0])
	return 2
}

func capsuleNew(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capsule new", flag.ContinueOnError)
	out := fs.String("o", "capsule.toml", "where to write the capsule")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	e, err := resolveHere()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rec, ok := box.ReadRunRecord(e.Scope.Key)
	if !ok {
		fmt.Fprintln(stderr, (&box.Error{
			Reason: "no run has been recorded for this sandbox yet",
			Cause:  "NO_RUN_RECORD", Scope: e.Scope,
			Fix: "run the failing command once (boxer run -c '<command>'), then boxer capsule new",
		}).Error())
		return 1
	}
	c := capsule{
		Version: 1, Created: time.Now().UTC(), Scope: rec.Scope, Image: rec.Image, Pack: rec.Pack,
		Run: capsuleRun{Task: rec.Task, Command: rec.Command, Dir: rec.Dir},
		Git: capsuleGit{Head: rec.GitHead, Dirty: rec.Dirty},
		Config: capsuleConfig{
			Image: e.Cfg.Image, ImageSetup: e.Cfg.ImageSetup, Setup: e.Cfg.Setup, Env: e.Cfg.Env,
			Mounts: e.Cfg.Mounts, NetworkMode: e.Cfg.Network.Mode, AllowHosts: e.Cfg.Network.AllowedHosts(),
		},
		Expect: capsuleExpect{Exit: rec.Exit},
	}
	if rec.Tests != nil {
		c.Expect.FailingTests = rec.Tests.Failed
	}
	// A dirty tree is the usual case for a failure worth capturing, and a capsule that quietly
	// dropped the uncommitted half would replay something that never failed.
	if rec.Dirty {
		patch, err := scope.GitCommand(e.Scope.Root, "diff", "HEAD").Output()
		if err == nil && len(patch) > 0 {
			p := patchPath(*out)
			if err := os.WriteFile(p, patch, 0o644); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			c.Git.Patch = filepath.Base(p)
			fmt.Fprintf(stdout, "boxer: wrote %s (%s)\n", p, box.HumanBytes(int64(len(patch))))
		}
	}
	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	err = toml.NewEncoder(f).Encode(c)
	// A capsule is written to be read by someone else, so a failed close is a failed write: the
	// encoder can succeed and the last buffer still not reach the disk.
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "boxer: wrote %s — exit %d from %q\n", *out, c.Expect.Exit, c.Run.Command)
	fmt.Fprintf(stdout, "boxer: replay it with `boxer capsule replay %s`\n", *out)
	return 0
}

func patchPath(out string) string {
	return strings.TrimSuffix(out, filepath.Ext(out)) + ".patch"
}

func capsuleInspect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capsule inspect", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the manifest as JSON")
	fs.SetOutput(stderr)
	arg, rest := firstOperand(args)
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	c, path, err := readCapsule(arg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if emit(stdout, c, *asJSON) {
		return 0
	}
	fmt.Fprintf(stdout, "capsule:   %s (version %d, %s)\n", path, c.Version, c.Created.Format(time.RFC3339))
	what := c.Run.Command
	if c.Run.Task != "" {
		what = "--task " + c.Run.Task + " (" + c.Run.Command + ")"
	}
	fmt.Fprintf(stdout, "command:   %s\ndir:       %s\nimage:     %s\n", what, c.Run.Dir, c.Image)
	fmt.Fprintf(stdout, "git:       %s%s\n", short(c.Git.Head), dirtyNote(c.Git))
	fmt.Fprintf(stdout, "expect:    exit %d", c.Expect.Exit)
	if n := len(c.Expect.FailingTests); n > 0 {
		fmt.Fprintf(stdout, ", %d failing test(s): %s", n, strings.Join(c.Expect.FailingTests, ", "))
	}
	fmt.Fprintln(stdout)
	return 0
}

func short(head string) string {
	if len(head) > 12 {
		return head[:12]
	}
	if head == "" {
		return "unknown"
	}
	return head
}

func dirtyNote(g capsuleGit) string {
	switch {
	case g.Patch != "":
		return " + " + g.Patch
	case g.Dirty:
		return " (was dirty; no patch captured)"
	}
	return ""
}

func capsuleReplay(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capsule replay", flag.ContinueOnError)
	allowDirty := fs.Bool("allow-dirty", false, "replay even though this worktree has uncommitted changes")
	allowDrift := fs.Bool("allow-drift", false, "replay even though HEAD differs from the one captured")
	applyPatch := fs.Bool("apply-patch", false, "apply the capsule's patch to this worktree before replaying")
	fs.SetOutput(stderr)
	arg, rest := firstOperand(args)
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	c, path, err := readCapsule(arg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	e, err := resolveHere()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	head, dirty := gitHere(e.Scope.Root, path, patchPath(path))
	if dirty && !*allowDirty {
		fmt.Fprintln(stderr, (&box.Error{
			Reason: "this worktree has uncommitted changes, and a replay is only meaningful against a known tree",
			Cause:  "DIRTY_WORKTREE", Scope: e.Scope,
			Fix: "commit or stash them, or `boxer capsule replay " + path + " --allow-dirty`",
		}).Error())
		return 1
	}
	if c.Git.Head != "" && head != "" && c.Git.Head != head && !*allowDrift {
		fmt.Fprintln(stderr, (&box.Error{
			Reason: fmt.Sprintf("the capsule was recorded at %s and this worktree is at %s", short(c.Git.Head), short(head)),
			Cause:  "CAPSULE_DRIFT", Scope: e.Scope,
			Fix: "git checkout " + short(c.Git.Head) + ", or `boxer capsule replay " + path + " --allow-drift`",
		}).Error())
		return 1
	}
	if c.Git.Patch != "" {
		if !*applyPatch {
			fmt.Fprintf(stderr, "boxer: this capsule carries %s, which is not applied; pass --apply-patch to replay the exact tree\n", c.Git.Patch)
		} else {
			p := filepath.Join(filepath.Dir(path), c.Git.Patch)
			cmd := exec.Command("git", "-C", e.Scope.Root, "apply", p)
			cmd.Stdout, cmd.Stderr = stdout, stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(stderr, "boxer: applying %s: %v\n", p, err)
				return 1
			}
			fmt.Fprintf(stdout, "boxer: applied %s\n", c.Git.Patch)
		}
	}
	for _, note := range applyCapsuleConfig(&e.Cfg, c.Config) {
		fmt.Fprintln(stderr, "boxer: "+note)
	}
	if err := e.Cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "boxer: the capsule's configuration is not valid: %v\n", err)
		return 1
	}
	if _, exists, _ := e.Exists(); exists {
		fmt.Fprintln(stderr, "boxer: a sandbox already exists here, so the capsule's image and setup apply only after `boxer down`")
	}
	e.CWD = filepath.Join(e.Scope.Root, filepath.FromSlash(c.Run.Dir))
	started := time.Now()
	fmt.Fprintf(stdout, "boxer: replaying %q in %s\n", c.Run.Command, c.Run.Dir)
	code, err := e.Run([]string{"sh", "-c", c.Run.Command}, box.RunOpts{Stdin: stdin, Stdout: stdout, Stderr: stderr})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return capsuleVerdict(e, c, code, started, stdout)
}

// capsuleVerdict compares the replay against the recorded outcome. Exit 0 means the failure
// reproduced, which is the opposite of the usual convention and is the whole point: a capsule is
// a question ("does this still fail?"), not a test suite.
func capsuleVerdict(e *box.Env, c capsule, code int, started time.Time, stdout io.Writer) int {
	same := code == c.Expect.Exit
	if !same {
		fmt.Fprintf(stdout, "boxer: did NOT reproduce — expected exit %d, got %d\n", c.Expect.Exit, code)
		return 1
	}
	if len(c.Expect.FailingTests) > 0 {
		s, err := junit.Collect(e.Scope.Root, junitPatterns(e.Cfg, config.Task{}, ""), started)
		if err == nil && len(s.Files) > 0 && !slices.Equal(s.Failed, c.Expect.FailingTests) {
			fmt.Fprintf(stdout, "boxer: did NOT reproduce — expected failures %s, got %s\n",
				strings.Join(c.Expect.FailingTests, ", "), strings.Join(s.Failed, ", "))
			return 1
		}
	}
	fmt.Fprintf(stdout, "boxer: reproduced — exit %d, as recorded\n", code)
	return 0
}

// applyCapsuleConfig lays a capsule's environment over the repository's. A capsule is a file
// someone attached to an issue, so nothing in it may reach further than the repository already
// does: its mounts (a host directory, ~/.ssh) are never applied, and its network only when it is
// no wider than this repository's. It returns what it left out.
func applyCapsuleConfig(cfg *config.Config, c capsuleConfig) (notes []string) {
	if c.Image != "" {
		// A capsule is someone else's file; a local-path image (a `docker save` archive or a
		// rootfs directory) would be a host path it chose, which it is not allowed to give.
		if box.IsLocalImage(c.Image) {
			notes = append(notes, "the capsule's image is a local path and is not applied; it names a file on this host")
		} else {
			cfg.Image = c.Image
		}
	}
	if len(c.ImageSetup) > 0 {
		cfg.ImageSetup = c.ImageSetup
	}
	if len(c.Setup) > 0 {
		cfg.Setup = c.Setup
	}
	if len(c.Env) > 0 {
		cfg.Env = c.Env
	}
	if len(c.Mounts) > 0 && !slices.Equal(c.Mounts, cfg.Mounts) {
		notes = append(notes, "the capsule's mounts are not applied: a capsule cannot give the sandbox host directories")
	}
	wider := map[string]int{"off": 0, "allowlist": 1, "on": 2}
	switch {
	case c.NetworkMode != "" && wider[c.NetworkMode] > wider[cfg.Network.Mode]:
		notes = append(notes, fmt.Sprintf("the capsule's network.mode %q is wider than this repository's %q and is not applied", c.NetworkMode, cfg.Network.Mode))
	case c.NetworkMode != "":
		cfg.Network.Mode = c.NetworkMode
	}
	if len(c.AllowHosts) > 0 {
		for _, h := range c.AllowHosts {
			if !slices.Contains(cfg.Network.AllowHosts, h) {
				notes = append(notes, "the capsule's allow_hosts name hosts this repository does not allow, and are not applied")
				return notes
			}
		}
		cfg.Network.AllowHosts = c.AllowHosts
	}
	return notes
}

func readCapsule(arg string) (capsule, string, error) {
	var c capsule
	if arg == "" {
		arg = "capsule.toml"
	}
	path := arg
	if st, err := os.Stat(arg); err == nil && st.IsDir() {
		path = filepath.Join(arg, "capsule.toml")
	}
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return c, path, fmt.Errorf("boxer capsule: %w", err)
	}
	if c.Version != 1 {
		return c, path, fmt.Errorf("%s: capsule version %d; this boxer writes and reads version 1", path, c.Version)
	}
	return c, path, nil
}

// resolveHere is the scope of the current directory, with no identity flags: a capsule belongs to
// a worktree, not to a session.
func resolveHere() (*box.Env, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return box.Resolve(cwd, "", scope.Identity{})
}

// firstOperand splits the first non-flag argument out of args, so `replay capsule.toml
// --allow-dirty` works: Go's flag package stops at the first operand, and a path that has to come
// before its flags is a trap nobody expects from a CLI.
func firstOperand(args []string) (string, []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			// The identity flags take a value; it is not the name. `pack save --harness claude
			// base` named the pack "claude".
			if f := strings.TrimLeft(a, "-"); !strings.Contains(f, "=") && (f == "harness" || f == "session" || f == "agent") {
				i++
			}
			continue
		}
		return a, append(append([]string{}, args[:i]...), args[i+1:]...)
	}
	return "", args
}

// gitHere reports HEAD and whether the worktree has changes, ignoring the capsule's own files:
// writing a capsule dirties the tree it describes, and refusing to replay because of that would
// make `capsule new && capsule replay` impossible.
func gitHere(root string, ignore ...string) (string, bool) {
	out, err := scope.GitCommand(root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", false
	}
	head := strings.TrimSpace(string(out))
	st, err := scope.GitCommand(root, "status", "--porcelain").Output()
	if err != nil {
		return head, false
	}
	// Each line is "XY path": the status is two columns, so the line is not trimmed (" M x"
	// would lose a letter of its path), and an ignored file is that exact file, not any file of
	// the same name elsewhere in the tree.
	for _, line := range strings.Split(strings.TrimRight(string(st), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		name := filepath.Join(root, line[3:])
		if !slices.ContainsFunc(ignore, func(ig string) bool {
			abs, err := filepath.Abs(ig)
			return ig != "" && err == nil && sameFile(abs, name)
		}) {
			return head, true
		}
	}
	return head, false
}

// sameFile compares two paths through symlinks (/var and /private/var on macOS).
func sameFile(a, b string) bool {
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	return a == b
}
