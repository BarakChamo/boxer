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
	if tier == "t2" {
		return false, "inside t2 needs each harness's login to travel; run `boxer shell <harness>` by hand"
	}
	if HostIP() == "" {
		return false, "no non-loopback IPv4 address for the guest to reach the fake model"
	}
	return true, ""
}

func (Inside) Cells(tier string) []Cell {
	var cells []Cell
	for _, h := range []string{"claude", "codex", "gemini", "kimi", "opencode", "pi"} {
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

// guestEnvFor is the per-harness environment that points the guest harness at its eval config
// directory (under the repo, so the path is the same on both sides) and at the fake model.
func (d Inside) guestEnvFor(env *Env, h string) []string {
	dir := d.cfgDir(env, h)
	url := env.LLMURLGuest
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
	switch h {
	case "claude":
		tr.Answer = parseClaudeStream(raw).Answer
	case "codex":
		tr.Answer = parseCodexJSON(raw).Answer
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
