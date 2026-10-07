// Package inside runs a harness itself in the worktree's VM: `boxer shell <harness>` for a human
// terminal, `boxer acp <harness>` for an ACP client. The harness does not know it is boxed; its
// shell, file tools, MCP servers, and hooks are all inside. Everything harness-specific here is a
// row of data, never a code path.
package inside

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/vm"
)

// Harness is one row of the table.
type Harness struct {
	Bin     string   // executable name inside the guest
	Install string   // shell line that installs it (node image)
	ACP     []string // argv for the ACP server, nil when the harness has none
	// ConfigVar and ConfigDir: the harness's config-directory variable and its host default. The
	// host directory is mounted read-write at the same path in the guest and the variable is set.
	ConfigVar string
	ConfigDir string
	Env       []string // host variables passed through when set (API keys, tokens, base URLs)
	// Creds are the Env entries that carry a login. When none is set and the harness's file login
	// does not travel with its config mount, LoginHint is printed once before the run.
	Creds     []string
	LoginHint string
	Hosts     []string // model API hosts the network allowlist must admit
	// The VM is the sandbox. GuestEnv and Args turn off the harness's own nested sandbox, which
	// cannot start inside the guest (Codex's bwrap fails on the mounted worktree) or, for Claude,
	// let it run as root. GuestEnv is set before caller extras; Args precede the user's arguments
	// in shell mode (ACP servers take the same through GuestEnv).
	GuestEnv []string
	Args     []string
}

// Harnesses is the table. The slim node image ships no CA store; harnesses that are Rust or Go
// binaries (Codex) need ca-certificates for TLS, node-based ones carry their own roots. Verified
// 2026-09-17 against installed CLIs: ACP commands exist for
// Gemini (--acp), Kimi (acp), OpenCode (acp), Grok (agent stdio); Claude and Codex through their
// ACP adapter packages; pi has none.
var Harnesses = map[string]Harness{
	"claude": {Bin: "claude", Install: "npm i -g @anthropic-ai/claude-code @agentclientprotocol/claude-agent-acp",
		ACP: []string{"claude-agent-acp"}, ConfigVar: "CLAUDE_CONFIG_DIR", ConfigDir: "~/.claude",
		Env:       []string{"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_MODEL"},
		Hosts:     []string{"api.anthropic.com", "claude.ai", "platform.claude.com"},
		GuestEnv:  []string{"IS_SANDBOX=1"},
		Creds:     []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"},
		LoginHint: "Claude Code's macOS Keychain login does not enter the VM; run `claude setup-token` on the host and export CLAUDE_CODE_OAUTH_TOKEN"},
	"codex": {Bin: "codex", Install: "apt-get update -qq && apt-get install -y -qq --no-install-recommends libssl3 ca-certificates && npm i -g @openai/codex @agentclientprotocol/codex-acp",
		ACP: []string{"codex-acp"}, ConfigVar: "CODEX_HOME", ConfigDir: "~/.codex",
		Env:      []string{"OPENAI_API_KEY", "OPENAI_BASE_URL"},
		Hosts:    []string{"api.openai.com", "chatgpt.com", "auth.openai.com"},
		GuestEnv: []string{`CODEX_CONFIG={"sandbox_mode":"danger-full-access"}`, "INITIAL_AGENT_MODE=agent-full-access"},
		Args:     []string{"-c", `sandbox_mode="danger-full-access"`}},
	// fx is a single native binary rather than an npm package, and its installer is a bash script:
	// it uses `set -o pipefail`, which the guest's /bin/sh (dash) rejects, so it is fetched and then
	// run under bash rather than piped into sh. It speaks the Vercel AI Gateway directly, so the
	// key is all it needs.
	"fx": {Bin: "fx", Install: "apt-get update -qq && apt-get install -y -qq --no-install-recommends curl ca-certificates && curl -fsSL https://fx.sh/setup.sh -o /tmp/fx-setup.sh && FX_INSTALL_DIR=/usr/local/bin bash /tmp/fx-setup.sh",
		ConfigVar: "FX_HOME", ConfigDir: "~/.fx",
		Env:   []string{"AI_GATEWAY_API_KEY", "FX_AI_GATEWAY_API_KEY", "FX_MODEL", "FX_PROVIDER"},
		Hosts: []string{"ai-gateway.vercel.sh", "fx.sh", "vercel.com"},
		Creds: []string{"AI_GATEWAY_API_KEY", "FX_AI_GATEWAY_API_KEY"},
		// fx keeps its credential in the macOS Keychain unless told not to; inside the guest there
		// is no Keychain, so the key has to come from the environment.
		GuestEnv:  []string{"FX_DISABLE_KEYCHAIN=1"},
		LoginHint: "fx reads AI_GATEWAY_API_KEY; export it on the host and it travels into the VM"},
	"gemini": {Bin: "gemini", Install: "npm i -g @google/gemini-cli",
		ACP: []string{"gemini", "--acp"}, ConfigVar: "GEMINI_CLI_HOME", ConfigDir: "~/.gemini",
		Env:   []string{"GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GEMINI_BASE_URL", "GEMINI_CLI_TRUST_WORKSPACE"},
		Hosts: []string{"generativelanguage.googleapis.com", "oauth2.googleapis.com", "accounts.google.com", "cloudcode-pa.googleapis.com"}},
	"kimi": {Bin: "kimi", Install: "npm i -g @moonshot-ai/kimi-code",
		ACP: []string{"kimi", "acp"}, ConfigVar: "KIMI_CODE_HOME", ConfigDir: "~/.kimi-code",
		Hosts: []string{"api.kimi.com", "api.moonshot.ai", "platform.kimi.ai", "code.kimi.com"}},
	"opencode": {Bin: "opencode", Install: "npm i -g opencode-ai",
		ACP: []string{"opencode", "acp"}, ConfigVar: "", ConfigDir: "~/.local/share/opencode",
		Env:   []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "XDG_CONFIG_HOME"},
		Hosts: []string{"api.anthropic.com", "api.openai.com", "models.dev", "opencode.ai"}},
	"pi": {Bin: "pi", Install: "npm i -g --ignore-scripts @earendil-works/pi-coding-agent",
		ConfigVar: "", ConfigDir: "~/.pi",
		Env:   []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "PI_SKIP_VERSION_CHECK", "PI_TELEMETRY"},
		Hosts: []string{"api.anthropic.com", "api.openai.com"}},
	// Copilot CLI verified 2026-09-18 against 1.0.86: `--acp` starts an ACP server, COPILOT_HOME
	// holds config and state, and the BYOK variables run it with no GitHub sign-in at all.
	// Copilot's model client is a native binary that reads the system trust store rather than
	// node's bundled one, and a slim image has no CA certificates: without them every request
	// fails with "No CA certificates were loaded from the system", which names neither TLS nor the
	// image. Codex needs them for the same reason.
	"copilot": {Bin: "copilot",
		Install: "apt-get update -qq && apt-get install -y -qq --no-install-recommends ca-certificates && npm i -g @github/copilot",
		ACP:     []string{"copilot", "--acp"}, ConfigVar: "COPILOT_HOME", ConfigDir: "~/.copilot",
		Env: []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN", "COPILOT_MODEL", "COPILOT_AUTO_UPDATE",
			"COPILOT_PROVIDER_TYPE", "COPILOT_PROVIDER_BASE_URL", "COPILOT_PROVIDER_API_KEY", "COPILOT_PROVIDER_WIRE_API"},
		Creds:     []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN", "COPILOT_PROVIDER_API_KEY"},
		LoginHint: "Copilot CLI's device login is stored under ~/.copilot, which is mounted, so it travels; otherwise export GH_TOKEN, or the BYOK trio COPILOT_PROVIDER_TYPE/BASE_URL/API_KEY",
		GuestEnv:  []string{"COPILOT_AUTO_UPDATE=false"},
		Hosts:     []string{"api.githubcopilot.com", "api.github.com", "github.com", "copilot-proxy.githubusercontent.com", "api.enterprise.githubcopilot.com"}},
	"grok": {Bin: "grok", Install: "npm i -g @xai-official/grok",
		ACP: []string{"grok", "agent", "stdio"}, ConfigVar: "GROK_HOME", ConfigDir: "~/.grok",
		Env:   []string{"XAI_API_KEY", "GROK_MODELS_BASE_URL"},
		Hosts: []string{"api.x.ai"}},
}

func init() {
	config.RegisterHarness(Names()...)
	// The programs inside mode runs, under the names their integrations have.
	config.RegisterHarnessAlias("claude", "claude-code")
	config.RegisterHarnessAlias("gemini", "gemini-cli")
	box.InsideHooks.Mounts = Mounts
	box.InsideHooks.AllowHosts = AllowHosts
	box.InsideHooks.Image = DefaultImage
	box.InsideHooks.InstallLine = func(h string) string { return Harnesses[h].Install }
}

// Names lists the harnesses in stable order.
func Names() []string {
	return []string{"claude", "codex", "fx", "gemini", "kimi", "opencode", "pi", "grok", "copilot"}
}

// DefaultImage is the guest image when the configuration names none: node for the npm harnesses,
// from the Google mirror to stay clear of Docker Hub's anonymous pull quota.
const DefaultImage = "mirror.gcr.io/library/node:24-bookworm-slim"

// Mounts returns host:guest volume specs for every known harness config directory that exists on
// the host, mounted at the same path so logins and sessions are shared with the host.
func Mounts() []string {
	var out []string
	seen := map[string]bool{}
	for _, name := range Names() {
		dir := configDir(Harnesses[name])
		if dir == "" || seen[dir] {
			continue
		}
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			out = append(out, dir+":"+dir)
			seen[dir] = true
		}
	}
	return out
}

// AllowHosts returns the model API hosts of every harness plus the npm registry.
func AllowHosts() []string {
	hosts := []string{"registry.npmjs.org", "deb.debian.org", "ai-gateway.vercel.sh"}
	for _, name := range Names() {
		hosts = append(hosts, Harnesses[name].Hosts...)
	}
	return hosts
}

func configDir(h Harness) string {
	if h.ConfigVar != "" {
		if v := os.Getenv(h.ConfigVar); v != "" {
			return v
		}
	}
	return expand(h.ConfigDir)
}

func expand(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// Options controls one run.
type Options struct {
	TTY    bool
	Env    []string // extra KEY=VALUE for the guest process
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Run ensures the VM and the harness, then executes argv (the harness binary plus args) inside.
func Run(e *box.Env, name string, args []string, acp bool, o Options) (int, error) {
	h, ok := Harnesses[name]
	if !ok {
		return 2, fmt.Errorf("unknown harness %q; known: %s", name, strings.Join(Names(), ", "))
	}
	if acp && h.ACP == nil {
		return 2, fmt.Errorf("%s has no ACP server; use `boxer shell %s`", name, name)
	}
	if hint := loginHint(h, o.Env); hint != "" {
		fmt.Fprintln(e.Stderr, "boxer: "+hint)
	}
	// Mounted only if it exists when the sandbox is made: a harness run before it ever logged in
	// on the host wrote its login to the guest's disk, gone with the sandbox.
	if dir := configDir(h); dir != "" {
		_ = os.MkdirAll(dir, 0o700)
	}
	created, err := e.Ensure(true, false)
	if err != nil {
		return 1, err
	}
	if err := install(e, name, h); err != nil {
		return 1, err
	}
	// Caching the harness stops and restarts the VM. Only one this call just made can be stopped
	// safely: another session, or a long `boxer run`, may be using an existing one, and the pack
	// cut it off. An existing VM's harness is cached the next time a sandbox is made for it.
	if created {
		e.PackHarness()
	}
	env, secrets, err := guestEnv(h, o.Env)
	if err != nil {
		return 1, err
	}
	// The repository's own [env], then the harness's: the harness session is where the agent runs
	// the repository's commands, so it gets what `boxer run` gives them.
	env = append(e.GuestEnv(), env...)
	// The runtime refuses a secret named twice, and the repository's secrets (CI in the default
	// env_passthrough, a name in `secrets`) can repeat one the harness or an -e already named.
	secrets = dedupeSecrets(append(secrets, e.HostSecrets()...))
	if m, ok, err := e.Exists(); err == nil && ok {
		env = append(env, e.ServerEnv(m)...)
	}
	argv := append(append([]string{h.Bin}, h.Args...), args...)
	if acp {
		argv = append(append([]string{}, h.ACP...), args...)
	}
	defer e.KeepAlive()()
	return e.VM.Exec(vm.ExecOpts{Name: e.Scope.Key, Workdir: e.GuestWorkdir(), Env: env, SecretEnv: secrets, TTY: o.TTY,
		User: e.Cfg.User, Stdin: o.Stdin, Stdout: o.Stdout, Stderr: o.Stderr}, argv...)
}

// install runs the harness's install line once per VM, recorded by a marker file. The marker
// travels in the harness pack, so VMs created from it skip this.
func transportError(stderr string) bool { return vm.TransportFailure(stderr) }

func install(e *box.Env, name string, h Harness) error {
	marker := "/var/lib/boxer/harness-" + name
	// A dropped transport is not an absent marker: read as one, it reinstalled the harness over an
	// install that was already there, which is npm in the guest for minutes.
	out, code, err := vm.Output(e.VM, e.Scope.Key, "", "sh", "-c", "test -f "+marker)
	if err != nil || transportError(out) {
		return &box.Error{Reason: "could not read the harness marker: " + strings.TrimSpace(out), Cause: "TRANSPORT_FAILED", Scope: e.Scope,
			Fix: "boxer up --recreate, then: boxer shell " + name}
	}
	if code == 0 {
		return nil
	}
	// Under the sandbox's lock: two sessions starting together ran two installs at once, and two
	// apt-based ones fought over dpkg's lock. The lock is released around a restart, which takes
	// it itself — holding it there deadlocked `boxer shell`.
	unlock, err := box.LockScope(e.Scope.Key)
	if err != nil {
		return err
	}
	defer func() { unlock() }() // unlock may be reassigned by the retry below
	// Someone else may have installed it while we waited for the lock.
	if _, c, _ := vm.Output(e.VM, e.Scope.Key, "", "sh", "-c", "test -f "+marker); c == 0 {
		return nil
	}
	fmt.Fprintf(e.Stderr, "boxer: installing %s in the sandbox\n", name)
	// npm inside the guest sees a slow registry through TSI: long fetch timeouts, and the whole
	// line retried once, cover the idle timeouts observed in eval runs.
	// npm exits 0 when an optional platform package fails to download, which leaves a harness
	// that cannot start; the check catches that before the marker is written and the VM packed.
	check := h.Bin + " --version >/dev/null 2>&1"
	if h.ACP != nil {
		// The ACP server has its own dependencies (Claude's native binary is an optional package);
		// `command -v` proves the entry point exists before the VM is packed and reused.
		check += " && command -v " + h.ACP[0] + " >/dev/null 2>&1"
	}
	// NPM_CONFIG_OMIT= keeps optional platform packages: a harness whose native binary is an
	// optional dependency (Claude's agent SDK, Codex's linux-arm64 build) installs with exit 0
	// without it and fails only when the model asks it to work, which a pack then preserves.
	line := "export NPM_CONFIG_FETCH_TIMEOUT=600000 NPM_CONFIG_FETCH_RETRIES=5 NPM_CONFIG_FETCH_RETRY_MAXTIMEOUT=120000 NPM_CONFIG_OMIT= DEBIAN_FRONTEND=noninteractive; " +
		"{ " + h.Install + " && " + check + "; } || { " + h.Install + " && " + check + "; }"
	run := func() (int, string, error) {
		var msg bytes.Buffer
		code, err := e.VM.Exec(vm.ExecOpts{Name: e.Scope.Key, Stdin: strings.NewReader(""), Stdout: e.Stderr, Stderr: io.MultiWriter(e.Stderr, &msg)},
			"sh", "-lc", line+" && mkdir -p /var/lib/boxer && touch "+marker)
		return code, msg.String(), err
	}
	code, msg, err := run()
	// A dropped exec transport is smolvm's, not npm's: restart the VM and run the whole line again.
	if (err != nil || code != 0) && transportError(msg) {
		fmt.Fprintln(e.Stderr, "boxer: the sandbox dropped the connection during install; restarting it and retrying once")
		unlock() // Restart provisions, which takes this lock
		rerr := e.Restart()
		unlock, err = box.LockScope(e.Scope.Key)
		if err != nil {
			return err
		}
		if rerr == nil {
			code, msg, err = run()
		}
	}
	if err != nil || code != 0 {
		return &box.Error{Reason: fmt.Sprintf("installing %s failed (exit %d): %s", name, code, LastLines(msg, 3)), Cause: "HARNESS_INSTALL_FAILED", Scope: e.Scope,
			Fix: "add registry.npmjs.org to network.allow_hosts, then: boxer up --recreate && boxer shell " + name}
	}
	return nil
}

// loginHint returns the harness's LoginHint when it has one and none of its Creds is set on the
// host or passed with -e.
func loginHint(h Harness, extra []string) string {
	for _, k := range h.Creds {
		if os.Getenv(k) != "" {
			return ""
		}
		for _, kv := range extra {
			if strings.HasPrefix(kv, k+"=") && len(kv) > len(k)+1 {
				return ""
			}
		}
	}
	return h.LoginHint
}

// guestEnv builds the guest environment: host HOME so mounted config paths resolve, the harness's
// config variable and caller extras last. The harness's own keys (API keys, OAuth tokens) are
// returned separately, as GUEST=HOSTVAR names, so the backend passes them without their values
// ever being in a command line; they used to be `-e KEY=value`, which any local user could read
// in the process list.
func guestEnv(h Harness, extra []string) (env, secrets []string, err error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// HOME=<empty> reaches the guest and every mounted config path resolves to "/", so the
		// harness starts with no login and no settings and says nothing useful about why.
		return nil, nil, fmt.Errorf("no home directory on the host, so the guest would get HOME=: %v", err)
	}
	env = []string{"HOME=" + home, "TERM=" + firstNonEmpty(os.Getenv("TERM"), "xterm-256color"), "LANG=C.UTF-8"}
	if h.ConfigVar != "" {
		env = append(env, h.ConfigVar+"="+configDir(h))
	}
	env = append(env, h.GuestEnv...)
	for _, k := range h.Env {
		if _, ok := os.LookupEnv(k); ok {
			secrets = append(secrets, k+"="+k)
		}
	}
	// -e values go the same way, by name: `-e CLAUDE_CODE_OAUTH_TOKEN=...` put the token on the
	// runtime's command line, which every local user can read. A bare -e NAME takes the host's.
	for _, kv := range extra {
		k, v, hasValue := strings.Cut(kv, "=")
		// One that overrides a variable set above (CODEX_HOME, HOME) replaces it there: that list
		// is passed by value, and a name passed both ways got the list's, not the caller's.
		if i := slices.IndexFunc(env, func(e string) bool { return strings.HasPrefix(e, k+"=") }); hasValue && i >= 0 {
			env[i] = kv
			continue
		}
		if hasValue {
			if err := os.Setenv(k, v); err != nil {
				return nil, nil, fmt.Errorf("-e %s: %v", k, err)
			}
		} else if _, ok := os.LookupEnv(k); !ok {
			continue
		}
		// Once: the runtime refuses a secret named twice, and the harness's own may name it too.
		if !slices.Contains(secrets, k+"="+k) {
			secrets = append(secrets, k+"="+k)
		}
	}
	return env, secrets, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// LastLines returns the final n non-empty lines of s, joined. It is what a failure has to carry:
// a process explains its death in its last words, and the whole log is too much for one line.
func LastLines(s string, n int) string {
	var keep []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			keep = append(keep, strings.TrimSpace(l))
		}
	}
	if len(keep) > n {
		keep = keep[len(keep)-n:]
	}
	return strings.Join(keep, "; ")
}

// dedupeSecrets keeps the first GUEST=HOST pair for each guest name: the runtime refuses two.
func dedupeSecrets(pairs []string) []string {
	seen := map[string]bool{}
	out := pairs[:0:0]
	for _, p := range pairs {
		name, _, _ := strings.Cut(p, "=")
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, p)
	}
	return out
}
