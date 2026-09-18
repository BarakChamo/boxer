package eval

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/inside"
)

// Inside drives `boxer shell <harness>`: the harness runs in the VM against the fake model. Its
// per-harness config lives under the repository (mounted at the same path in the guest), so each
// harness's config-dir variable points at a path valid on both sides. No hooks, no rewrite: the
// only assertion is that the harness's own shell ran in the guest.
type Inside struct{}

func (Inside) Name() string { return "inside" }

func (Inside) Available(tier string) (bool, string) {
	if tier == "t1" && HostIP() == "" {
		return false, "no non-loopback IPv4 address for the guest to reach the fake model"
	}
	return true, ""
}

func (Inside) Cells(tier string) []Cell {
	var cells []Cell
	// Copilot joins the inside cells: its row existed in the table and had never been run, which
	// is a claim without evidence. DSH has no inside row at all — it is a plugin stack rather than
	// a single binary — so it is absent here rather than silently expected to work.
	for _, h := range []string{"claude", "codex", "gemini", "kimi", "opencode", "pi", "grok", "copilot"} {
		cells = append(cells, Cell{Harness: "inside-" + h, Mode: "inside", Entry: "shell", Isolation: "worktree", Compliant: true, Tier: tier, Inside: h})
	}
	return cells
}

// HostIP is an address of this machine the guest can reach (smolvm TSI routes guest traffic
// through the host, so the host's LAN address works; loopback does not).
func HostIP() string {
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() && ipn.IP.To4() != nil && !ipn.IP.IsLinkLocalUnicast() {
			return ipn.IP.String()
		}
	}
	return ""
}

func (d Inside) cfgDir(env *Env, h string) string { return filepath.Join(env.Repo, ".boxer-eval", h) }

func (d Inside) Prepare(env *Env, c Cell) error {
	h := c.Inside
	dir := d.cfgDir(env, h)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if env.Tier == "t2" {
		return d.prepareLive(env, h, dir)
	}
	url := env.LLMURLGuest
	var files map[string]string
	switch h {
	case "claude":
		files = map[string]string{".claude.json": fmt.Sprintf(`{"customApiKeyResponses":{"approved":[%q],"rejected":[]},"hasCompletedOnboarding":true,"theme":"dark","numStartups":3}`, fakeKey[len(fakeKey)-20:])}
	case "codex":
		files = map[string]string{"config.toml": fmt.Sprintf("model = \"fake-model\"\nmodel_provider = \"fake\"\n[model_providers.fake]\nname = \"fake\"\nbase_url = \"%s/v1\"\nwire_api = \"responses\"\nrequires_openai_auth = false\n", url)}
		files["config.toml"] = "approval_policy = \"never\"\n" + files["config.toml"] // sandbox_mode comes from boxer's harness table
	case "gemini":
		files = map[string]string{".gemini/settings.json": `{"security":{"auth":{"selectedType":"gemini-api-key"}},"general":{"disableAutoUpdate":true,"disableUpdateNag":true},"privacy":{"usageStatisticsEnabled":false}}`}
	case "kimi":
		files = map[string]string{"config.toml": fmt.Sprintf("default_model = \"fake\"\ndefault_permission_mode = \"auto\"\ntelemetry = false\n[providers.fake]\ntype = \"anthropic\"\napi_key = \"fake\"\nbase_url = %q\n[models.fake]\nprovider = \"fake\"\nmodel = \"fake-model\"\nmax_context_size = 200000\n", url)}
	case "opencode":
		oc := fmt.Sprintf(`{"$schema":"https://opencode.ai/config.json","model":"fake/fake-model","permission":{"bash":"allow","edit":"allow","external_directory":"allow"},"provider":{"fake":{"npm":"@ai-sdk/openai-compatible","name":"fake","options":{"baseURL":"%s/v1","apiKey":"fake"},"models":{"fake-model":{"name":"fake-model","tool_call":true}}}}}`, url)
		if err := os.WriteFile(filepath.Join(env.Repo, "opencode.json"), []byte(oc+"\n"), 0o644); err != nil {
			return err
		}
	case "grok":
		// Grok's own sandbox is off by default (R-INT-4a), so nothing turns it off here.
		files = map[string]string{"config.toml": fmt.Sprintf("[models]\ndefault = \"fake-model\"\n[cli]\nauto_update = false\n[features]\ntelemetry = \"off\"\n[model.fake-model]\nmodel = \"fake-model\"\nname = \"fake\"\nbase_url = \"%s/v1\"\napi_key = \"fake\"\napi_backend = \"chat_completions\"\ncontext_window = 128000\n", url)}
	case "pi":
		files = map[string]string{".pi/agent/models.json": fmt.Sprintf(`{"providers":{"fake":{"baseUrl":%q,"api":"anthropic-messages","apiKey":"fake","models":[{"id":"fake-model","name":"fake","contextWindow":200000,"maxTokens":8192,"input":["text"],"reasoning":false}]}}}`, url)}
	}
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body+"\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// prepareLive writes the t2 config for one guest harness: the real provider, keyed from the
// environment. A harness whose credential is absent skips and names it.
func (d Inside) prepareLive(env *Env, h, dir string) error {
	files := map[string]string{}
	key, why := gatewayKey()
	if why != "" && h != "gemini" {
		return SkipError{"inside " + h + ": " + why}
	}
	switch h {
	case "claude":
		files[".claude.json"] = fmt.Sprintf(`{"customApiKeyResponses":{"approved":[%q],"rejected":[]},"hasCompletedOnboarding":true,"theme":"dark","numStartups":3}`, key[len(key)-20:])
	case "codex":
		files["config.toml"] = "approval_policy = \"never\"\n" + codexConfig(env)
	case "grok":
		files["config.toml"] = "[cli]\nauto_update = false\n[features]\ntelemetry = \"off\"\n" + grokModel(env)
	case "gemini":
		if why := needOne("GEMINI_API_KEY", "GOOGLE_API_KEY"); why != "" {
			return SkipError{"inside gemini: " + why}
		}
		files[".gemini/settings.json"] = `{"security":{"auth":{"selectedType":"gemini-api-key"}},"general":{"disableAutoUpdate":true,"disableUpdateNag":true},"privacy":{"usageStatisticsEnabled":false}}`
	case "kimi":
		files["config.toml"] = kimiConfig("anthropic", gatewayRoot, key, LiveModel("kimi"))
	case "opencode", "pi":
		p, _ := gateway(h)
		if h == "opencode" {
			oc := fmt.Sprintf(`{"$schema":"https://opencode.ai/config.json","model":"eval/%s","permission":{"bash":"allow","edit":"allow","external_directory":"allow"},"provider":{"eval":{"npm":"@ai-sdk/openai-compatible","name":"eval","options":{"baseURL":%q,"apiKey":%q},"models":{%q:{"name":%q,"tool_call":true}}}}}`, p.Model, p.BaseURL, os.Getenv(p.KeyVar), p.Model, p.Model)
			return os.WriteFile(filepath.Join(env.Repo, "opencode.json"), []byte(oc+"\n"), 0o644)
		}
		files[".pi/agent/models.json"] = fmt.Sprintf(`{"providers":{"eval":{"baseUrl":%q,"api":"openai-completions","apiKey":%q,"models":[{"id":%q,"name":%q,"contextWindow":200000,"maxTokens":8192,"input":["text"],"reasoning":false}]}}}`, p.BaseURL, os.Getenv(p.KeyVar), p.Model, p.Model)
	}
	for rel, body := range files {
		path := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(body+"\n"), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// guestEnvFor is the per-harness environment that points the guest harness at its eval config
// directory (under the repo, so the path is the same on both sides) and at the fake model, or at
// the real provider at t2.
func (d Inside) guestEnvFor(env *Env, h string) []string {
	dir := d.cfgDir(env, h)
	url := env.LLMURLGuest
	if env.Tier == "t2" {
		key, _ := gatewayKey()
		switch h {
		case "claude":
			e := []string{"CLAUDE_CONFIG_DIR=" + dir, "ANTHROPIC_BASE_URL=" + gatewayClaude, "ANTHROPIC_API_KEY=" + key, "ANTHROPIC_MODEL=" + LiveModel("claude"), "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1"}
			if !strings.HasPrefix(LiveModel("claude"), "anthropic/") {
				e = append(e, "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1")
			}
			return e
		case "codex":
			return []string{"CODEX_HOME=" + dir, gatewayKeyVar + "=" + key}
		case "grok":
			return []string{"GROK_HOME=" + dir, "GROK_DISABLE_AUTOUPDATER=1", gatewayKeyVar + "=" + key}
		case "gemini":
			k, _ := anySet("GEMINI_API_KEY", "GOOGLE_API_KEY")
			return []string{"GEMINI_CLI_HOME=" + dir, "GEMINI_API_KEY=" + os.Getenv(k), "GEMINI_CLI_TRUST_WORKSPACE=true"}
		case "copilot":
			return append([]string{"COPILOT_HOME=" + dir, "COPILOT_AUTO_UPDATE=false", "COPILOT_ALLOW_ALL=true"},
				"COPILOT_PROVIDER_TYPE=openai", "COPILOT_PROVIDER_BASE_URL="+gatewayOpenAI,
				"COPILOT_PROVIDER_API_KEY="+key, "COPILOT_MODEL="+LiveModel("copilot"))
		}
	}
	switch h {
	case "claude":
		return []string{"CLAUDE_CONFIG_DIR=" + dir, "ANTHROPIC_BASE_URL=" + url, "ANTHROPIC_API_KEY=" + fakeKey, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1"}
	case "codex":
		return []string{"CODEX_HOME=" + dir}
	case "gemini":
		return []string{"GEMINI_CLI_HOME=" + dir, "GEMINI_API_KEY=fake", "GOOGLE_GEMINI_BASE_URL=" + url, "GEMINI_CLI_TRUST_WORKSPACE=true"}
	case "kimi":
		return []string{"KIMI_CODE_HOME=" + dir}
	case "opencode":
		return []string{"XDG_DATA_HOME=" + filepath.Join(dir, "data"), "XDG_CONFIG_HOME=" + filepath.Join(dir, "config"), "XDG_CACHE_HOME=" + filepath.Join(dir, "cache")}
	case "pi":
		return []string{"HOME=" + dir, "PI_SKIP_VERSION_CHECK=1", "PI_TELEMETRY=0"}
	case "grok":
		return []string{"GROK_HOME=" + dir, "GROK_DISABLE_AUTOUPDATER=1"}
	case "copilot":
		// COPILOT_ALLOW_ALL trusts the working directory, which headless mode otherwise refuses.
		return []string{"COPILOT_HOME=" + dir, "COPILOT_AUTO_UPDATE=false", "COPILOT_ALLOW_ALL=true",
			"COPILOT_PROVIDER_TYPE=openai", "COPILOT_PROVIDER_BASE_URL=" + url + "/v1",
			"COPILOT_PROVIDER_API_KEY=fake", "COPILOT_MODEL=fake-model"}
	}
	return nil
}

func (d Inside) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	h := c.Inside
	envs := d.guestEnvFor(env, h)
	var args []string
	switch h {
	case "claude":
		args = []string{"-p", prompt, "--permission-mode", "bypassPermissions", "--output-format", "stream-json", "--verbose", "--max-turns", "6"}
	case "codex":
		args = []string{"exec", "--skip-git-repo-check", "--json", prompt} // no bypass flag: the table's sandbox_mode and the config's approval_policy carry it
	case "gemini":
		args = []string{"-p", prompt, "--yolo"}
	case "kimi":
		args = []string{"-p", prompt}
	case "opencode":
		args = []string{"run", "-m", "fake/fake-model", prompt}
	case "pi":
		args = []string{"-p", "--no-session", "--provider", "fake", "--model", "fake-model", prompt}
	case "grok":
		args = []string{"-p", prompt, "-m", "fake-model", "--permission-mode", "bypassPermissions", "--no-auto-update"}
	case "copilot":
		// Inside the guest the sandbox is the machine, so the permissions that matter are the
		// harness's own. --allow-all-tools is refused where a GitHub policy says so, hence the
		// named grants; --allow-all-paths because Copilot verifies the paths a command touches.
		args = []string{"-p", prompt, "-s", "--allow-tool", "shell", "--allow-tool", "write", "--allow-all-paths",
			"--no-ask-user", "--output-format", "json", "--no-auto-update"}
	}
	if env.Tier == "t2" {
		switch h {
		case "opencode":
			args = []string{"run", "-m", "eval/" + LiveModel("opencode"), prompt}
		case "pi":
			args = []string{"-p", "--no-session", "--provider", "eval", "--model", LiveModel("pi"), prompt}
		case "grok":
			args = []string{"-p", prompt, "-m", "live", "--permission-mode", "bypassPermissions", "--no-auto-update"}
		}
	}
	cmdArgs := []string{"shell", h}
	for _, e := range envs {
		cmdArgs = append(cmdArgs, "-e", e)
	}
	cmdArgs = append(cmdArgs, "--")
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(env.Boxer, cmdArgs...)
	cmd.Dir = env.Repo
	cmd.Env = env.BaseEnv()
	cmd.Stdin = strings.NewReader("")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return Transcript{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(15 * time.Minute): // fresh VM (~75s), harness install (~80s), then the run
		cmd.Process.Kill()
		return Transcript{Raw: out.String()}, fmt.Errorf("boxer shell %s timed out", h)
	}
	raw := out.String()
	tr := Transcript{Raw: raw, Tools: env.LLMTools()}
	if env.Tier == "t2" {
		if q := quotaError(raw); q != "" {
			return tr, SkipError{q}
		}
	}
	switch h {
	case "claude":
		tr.Answer = parseClaudeStream(raw).Answer
	case "codex":
		tr.Answer = parseCodexJSON(raw).Answer
	case "copilot":
		tr.Answer = parseCopilotJSON(raw).Answer
	default:
		for _, line := range strings.Split(stripANSI(raw), "\n") {
			l := strings.TrimSpace(line)
			if l == "" || strings.HasPrefix(l, "boxer") || strings.HasPrefix(l, "<") || strings.HasPrefix(l, "$ ") || strings.HasPrefix(l, "timestamp=") || strings.HasPrefix(l, "YOLO") || strings.HasPrefix(l, "To resume") || strings.HasPrefix(l, "[") || strings.HasPrefix(l, "Warning") || strings.HasPrefix(l, "npm") || strings.HasPrefix(l, "added") || strings.HasPrefix(l, "changed") {
				continue
			}
			l = strings.TrimPrefix(l, "• ")
			if f := strings.Fields(l); len(f) > 0 {
				tr.Answer = f[0]
			}
		}
	}
	return tr, nil
}

func (Inside) Cleanup(env *Env, c Cell) {}

var _ = inside.Names // the driver mirrors the harness table; keep the dependency explicit
