// Package inside runs a harness itself in the worktree's VM: `boxer shell <harness>` for a human
// terminal, `boxer acp <harness>` for an ACP client. The harness does not know it is boxed; its
// shell, file tools, MCP servers, and hooks are all inside. Everything harness-specific here is a
// row of data, never a code path.
package inside

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BarakChamo/boxer/internal/box"
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

// Harnesses is the table. Verified 2026-09-17 against installed CLIs: ACP commands exist for
// Gemini (--acp), Kimi (acp), OpenCode (acp), Grok (agent stdio); Claude and Codex through their
// ACP adapter packages; pi has none.
var Harnesses = map[string]Harness{
	"claude": {Bin: "claude", Install: "npm i -g @anthropic-ai/claude-code @agentclientprotocol/claude-agent-acp",
		ACP: []string{"claude-agent-acp"}, ConfigVar: "CLAUDE_CONFIG_DIR", ConfigDir: "~/.claude",
		Env:       []string{"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_MODEL"},
		Hosts:     []string{"api.anthropic.com", "claude.ai", "platform.claude.com", "statsig.anthropic.com"},
		GuestEnv:  []string{"IS_SANDBOX=1"},
		Creds:     []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"},
		LoginHint: "Claude Code's macOS Keychain login does not enter the VM; run `claude setup-token` on the host and export CLAUDE_CODE_OAUTH_TOKEN"},
	"codex": {Bin: "codex", Install: "apt-get update -qq && apt-get install -y -qq --no-install-recommends libssl3 && npm i -g @openai/codex @agentclientprotocol/codex-acp",
		ACP: []string{"codex-acp"}, ConfigVar: "CODEX_HOME", ConfigDir: "~/.codex",
		Env:      []string{"OPENAI_API_KEY", "OPENAI_BASE_URL"},
		Hosts:    []string{"api.openai.com", "chatgpt.com", "auth.openai.com"},
		GuestEnv: []string{`CODEX_CONFIG={"sandbox_mode":"danger-full-access"}`, "INITIAL_AGENT_MODE=agent-full-access"},
		Args:     []string{"-c", `sandbox_mode="danger-full-access"`}},
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
	"grok": {Bin: "grok", Install: "npm i -g @xai-official/grok",
		ACP: []string{"grok", "agent", "stdio"}, ConfigVar: "GROK_HOME", ConfigDir: "~/.grok",
		Env:   []string{"XAI_API_KEY", "GROK_MODELS_BASE_URL"},
		Hosts: []string{"api.x.ai"}},
}

func init() {
	box.InsideHooks.Mounts = Mounts
	box.InsideHooks.AllowHosts = AllowHosts
	box.InsideHooks.Image = DefaultImage
}

// Names lists the harnesses in stable order.
func Names() []string {
	return []string{"claude", "codex", "gemini", "kimi", "opencode", "pi", "grok"}
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
	hosts := []string{"registry.npmjs.org", "deb.debian.org"}
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
	if _, err := e.Ensure(true, false); err != nil {
		return 1, err
	}
	if err := install(e, name, h); err != nil {
		return 1, err
	}
	e.PackHarness()
	env := guestEnv(h, o.Env)
	argv := append(append([]string{h.Bin}, h.Args...), args...)
	if acp {
		argv = append(append([]string{}, h.ACP...), args...)
	}
	return e.VM.Exec(vm.ExecOpts{Name: e.Scope.Key, Workdir: e.GuestWorkdir(), Env: env, TTY: o.TTY,
		Stdin: o.Stdin, Stdout: o.Stdout, Stderr: o.Stderr}, argv...)
}

// install runs the harness's install line once per VM, recorded by a marker file. The marker
// travels in the harness pack, so VMs created from it skip this.
func install(e *box.Env, name string, h Harness) error {
	marker := "/var/lib/boxer/harness-" + name
	if _, code, _ := e.VM.Output(e.Scope.Key, "", "sh", "-c", "test -f "+marker); code == 0 {
		return nil
	}
	fmt.Fprintf(e.Stderr, "boxer: installing %s in the sandbox (once per host)\n", name)
	// npm inside the guest sees a slow registry through TSI: long fetch timeouts, and the whole
	// line retried once, cover the idle timeouts observed in eval runs.
	line := "export NPM_CONFIG_FETCH_TIMEOUT=600000 NPM_CONFIG_FETCH_RETRIES=5 NPM_CONFIG_FETCH_RETRY_MAXTIMEOUT=120000 DEBIAN_FRONTEND=noninteractive; " +
		"{ " + h.Install + "; } || { " + h.Install + "; }"
	code, err := e.VM.Exec(vm.ExecOpts{Name: e.Scope.Key, Stdin: strings.NewReader(""), Stdout: e.Stderr, Stderr: e.Stderr},
		"sh", "-lc", line+" && mkdir -p /var/lib/boxer && touch "+marker)
	if err != nil || code != 0 {
		return &box.Error{Reason: fmt.Sprintf("installing %s failed (exit %d)", name, code), Cause: "HARNESS_INSTALL_FAILED", Scope: e.Scope,
			Fix: "check network.allow_hosts includes registry.npmjs.org, then: boxer shell " + name}
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
// config variable, passthrough of its keys when set on the host, and caller extras last.
func guestEnv(h Harness, extra []string) []string {
	home, _ := os.UserHomeDir()
	env := []string{"HOME=" + home, "TERM=" + firstNonEmpty(os.Getenv("TERM"), "xterm-256color"), "LANG=C.UTF-8"}
	if h.ConfigVar != "" {
		env = append(env, h.ConfigVar+"="+configDir(h))
	}
	env = append(env, h.GuestEnv...)
	for _, k := range h.Env {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return append(env, extra...)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
