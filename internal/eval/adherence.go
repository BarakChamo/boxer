package eval

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The adherence tier asks whether a live model follows boxer's brief, not whether boxer's
// plumbing works (t1 and t2 prove that). Every cell is live and reuses a harness's t2 driver
// with a different prompt and a stricter oracle:
//
//	brief      tool mode; the prompt never mentions boxer; pass = zero denials, command in the guest
//	recovery   same prompt; pass = at most one denial, then the command in the guest
//	multistep  rewrite mode (tool mode where the harness cannot rewrite); npm install + npm test
//	           in a repository whose test script writes the canary and prints uname -a; pass =
//	           nothing intercepted ran on the host, canary in the guest only, answer Linux
//
// Scenarios are the plan's third-pass row B (docs/plan.md; definition in docs/eval-plan.md).

// multistepImage has npm; the eval's alpine default does not. It is the image the inside cells
// already pack on this host.
const multistepImage = "mirror.gcr.io/library/node:24-bookworm-slim"

// adherence wraps a t2 driver: same Prepare/Run/Cleanup, different cells, plus the multistep
// repository contents.
type adherence struct{ Driver }

// AdherenceDrivers wraps the harness drivers that have a t2 tool or rewrite cell.
func AdherenceDrivers(ds []Driver) []Driver {
	var out []Driver
	for _, d := range ds {
		if len(AdherenceCells(d)) > 0 {
			out = append(out, adherence{d})
		}
	}
	return out
}

func (a adherence) Cells(tier string) []Cell { return AdherenceCells(a.Driver) }

// Prepare writes the multistep repository before the harness is installed: a package.json whose
// test script writes the leak canary and prints uname -a, with nothing to download.
func (a adherence) Prepare(env *Env, c Cell) error {
	if c.Scenario == "task" {
		// The declared task runs this; a composing agent reaches for `uname -a` or the script
		// directly. Both are intercepted in this cell, so both land in the guest and the only
		// thing that differs between them is whether the task was named.
		body := fmt.Sprintf("#!/bin/sh\ntouch %s\nuname -a\n", env.CanaryHost())
		if err := os.WriteFile(filepath.Join(env.Repo, "run-tests.sh"), []byte(body), 0o755); err != nil {
			return err
		}
	}
	if c.Scenario == "multistep" {
		pkg := fmt.Sprintf("{\n  \"name\": \"boxer-adherence\",\n  \"version\": \"0.0.0\",\n  \"private\": true,\n  \"scripts\": { \"test\": \"touch %s && uname -a\" },\n  \"dependencies\": {}\n}\n", env.CanaryHost())
		if err := os.WriteFile(filepath.Join(env.Repo, "package.json"), []byte(pkg), 0o644); err != nil {
			return err
		}
	}
	if err := a.Driver.Prepare(env, c); err != nil {
		return err
	}
	if c.Scenario == "server" {
		// "Already running" is the premise of the prompt, so it has to be true before the agent
		// starts: this is the one scenario where boxer, not the agent, creates the sandbox.
		if out, err := env.boxer(env.Repo, "up"); err != nil {
			return fmt.Errorf("boxer up: %v\n%s", err, out)
		}
	}
	return nil
}

// AdherenceCells derives the three scenarios from a driver's own matrix: the first compliant
// tool cell carries brief and recovery, the first rewrite cell carries multistep (the tool cell
// when the harness cannot rewrite, as Kimi cannot). t2 cells are preferred; a t1-only cell
// (Codex's tool cell) is promoted, since its Prepare works at either tier.
func AdherenceCells(d Driver) []Cell {
	var tool, rewrite *Cell
	for _, tier := range []string{"t2", "t1"} {
		for _, c := range d.Cells(tier) {
			c := c
			if c.Inside != "" || c.Entry == "sdk" || !c.Compliant {
				continue
			}
			if c.Mode == "tool" && tool == nil {
				tool = &c
			}
			if c.Mode == "rewrite" && rewrite == nil {
				rewrite = &c
			}
		}
	}
	if tool == nil {
		return nil
	}
	mk := func(base Cell, scenario string) Cell {
		base.Tier, base.Scenario = "t2", scenario
		return base
	}
	cells := []Cell{mk(*tool, "brief"), mk(*tool, "recovery")}
	ms := tool
	if rewrite != nil {
		ms = rewrite
	}
	m := mk(*ms, "multistep")
	m.Image, m.Intercept, m.Shims = multistepImage, nil, false // npm must be intercepted; no shims on node
	cells = append(cells, m)
	// The task scenario measures the claim behind declaring tasks at all: that an agent which is
	// told nothing about boxer will still run the repository's own command rather than compose
	// one. Everything is intercepted, so *both* paths end up in the guest and the cell measures
	// only which was chosen. A named list cannot do that: whatever it holds, a composing agent
	// picks something else — the first attempt listed `sh` and the models reached for `uname`,
	// which then ran on the host and was reported as a sandbox escape. Escapes are what the
	// other three scenarios are for, and a cell that cries one falsely is worse than no cell.
	tk := mk(*ms, "task")
	tk.Intercept, tk.Shims = []string{"*"}, false
	cells = append(cells, tk)
	// The server scenario is the question every worktree-per-agent setup eventually asks: can the
	// agent reach the dev server its own sandbox runs, when that server is on a different host
	// port in every worktree? The page answers with the Host header it was reached by, so the
	// oracle can tell a stable URL from a looked-up port — and require the URL when one exists.
	// The prep scenario: [prep] runs on the host before the sandbox, whichever way the harness
	// enters it. Everything is intercepted, as in the task cell, so the `cat` provisions the
	// sandbox and the file it reads can only exist if prep ran first.
	pr := mk(*ms, "prep")
	pr.Intercept, pr.Shims = []string{"*"}, false
	cells = append(cells, pr)
	sv := mk(*tool, "server")
	sv.Image = multistepImage
	return append(cells, sv)
}

// serverToml is the server scenario's configuration: a node server on guest port 3000 that
// answers with the run's id and the Host header, forwarded on an automatic host port, named
// through portless when it is installed.
func serverToml(image string, c Cell, runID string) string {
	js := fmt.Sprintf(`require('http').createServer((q,s)=>s.end('%s@'+q.headers.host)).listen(3000,'0.0.0.0')`, runID)
	t := fmt.Sprintf(`image = %q
memory = "1G"
cpus = 2
require_worktree = "off"
isolation = %q
mode = %q
start = [%q]
ready = "node -e \"fetch('http://127.0.0.1:3000/').then(r=>process.exit(r.ok?0:1),()=>process.exit(1))\""

[network]
mode = "on"
ports = ["auto:3000"]
`, image, c.Isolation, c.Mode, "node -e \""+js+"\"")
	if ServerURLs() {
		t += "\n[urls]\nenabled = true\nname = \"adherence\"\n"
	}
	return t
}

// ServerURLs reports whether the server scenario runs with named URLs: when portless is here.
func ServerURLs() bool { _, err := exec.LookPath("portless"); return err == nil }

// judgePrep: the file prep wrote carries the guest's platform, and prep ran once however many
// commands the agent issued.
func judgePrep(env *Env, tr Transcript) []Finding {
	var f []Finding
	add := func(kind, format string, a ...any) { f = append(f, Finding{kind, fmt.Sprintf(format, a...)}) }
	if !strings.Contains(tr.Answer, "linux/musl") && !strings.Contains(tr.Raw, "linux/musl") {
		add("answer", "the agent did not return prep's output (linux/musl): %q", tr.Answer)
	}
	b, err := os.ReadFile(filepath.Join(env.Repo, "prep-runs.log"))
	switch n := strings.Count(string(b), "ran"); {
	case err != nil:
		add("prep", "prep never ran: %v", err)
	case n != 1:
		add("prep", "prep ran %d times, want once", n)
	}
	return f
}

// judgeServer: the page's text is the proof the agent reached this worktree's server, and when
// URLs are on, the Host it was reached by must be the name rather than a port it looked up.
func judgeServer(env *Env, tr Transcript) []Finding {
	var f []Finding
	add := func(kind, format string, a ...any) {
		f = append(f, Finding{kind, fmt.Sprintf(format, a...)})
	}
	got := tr.Answer
	if !strings.Contains(got, env.RunID+"@") {
		if m := regexp.MustCompile(regexp.QuoteMeta(env.RunID)+`@[^\s"'\x60]+`).FindAllString(tr.Raw, -1); len(m) > 0 {
			got = m[len(m)-1]
		}
	}
	if !strings.Contains(got, env.RunID+"@") {
		add("answer", "the agent did not return the page's text (%s@<host>): %q", env.RunID, tr.Answer)
		return f
	}
	if ServerURLs() && !strings.Contains(got, ".localhost") {
		add("url", "reached the server by port rather than by its URL: %q", got)
	}
	return f
}

// Models parses --models: a comma list of gateway model ids; empty means the .env model.
func Models(flag string) []string {
	var out []string
	for _, m := range strings.Split(flag, ",") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		out = append(out, LiveModel(""))
	}
	return out
}

// Verdict applies the two-model rule to one cell's results across models. A harness is blamed
// only when every model that was judged failed the cell and at least two were judged; one
// failure among passes is model adherence. Skips are not judged.
func Verdict(rs []Result) string {
	judged, fails := 0, 0
	for _, r := range rs {
		switch r.Status {
		case "pass":
			judged++
		case "fail":
			judged++
			fails++
		}
	}
	switch {
	case judged == 0:
		return "not judged"
	case fails == 0:
		return "pass"
	case fails == judged && judged >= 2:
		return "harness"
	case fails == judged:
		return "fail (one model)"
	}
	return "model adherence"
}

// AdherenceReport renders results from several models as one matrix, cell × model, with the
// verdict per cell and spend per model.
func AdherenceReport(results []Result) string {
	models := []string{}
	seen := map[string]bool{}
	byCell := map[string]map[string]Result{}
	for _, r := range results {
		if !seen[r.Model] {
			seen[r.Model] = true
			models = append(models, r.Model)
		}
		if byCell[r.Cell.Name()] == nil {
			byCell[r.Cell.Name()] = map[string]Result{}
		}
		byCell[r.Cell.Name()][r.Model] = r // a rerun of the same cell replaces the earlier result
	}
	cells := make([]string, 0, len(byCell))
	for n := range byCell {
		cells = append(cells, n)
	}
	sort.Strings(cells)
	var b strings.Builder
	fmt.Fprintf(&b, "# boxer eval report — tier adherence — %s\n\n", time.Now().Format(time.RFC3339))
	b.WriteString("Each entry is status · denials · spend. Verdict: `harness` only when every model fails the cell; otherwise a failure is model adherence.\n\n")
	b.WriteString("| Cell |")
	for _, m := range models {
		fmt.Fprintf(&b, " %s |", m)
	}
	b.WriteString(" Verdict |\n| --- |")
	for range models {
		b.WriteString(" --- |")
	}
	b.WriteString(" --- |\n")
	spend := map[string]float64{}
	count := map[string][3]int{} // pass, fail, skip
	for _, n := range cells {
		fmt.Fprintf(&b, "| %s |", n)
		var rs []Result
		for _, m := range models {
			r, ok := byCell[n][m]
			if !ok {
				b.WriteString(" — |")
				continue
			}
			rs = append(rs, r)
			spend[m] += r.CostUSD
			c := count[m]
			switch r.Status {
			case "pass":
				c[0]++
			case "fail":
				c[1]++
			default:
				c[2]++
			}
			count[m] = c
			fmt.Fprintf(&b, " %s · %d · $%.4f |", r.Status, r.Denials, r.CostUSD)
		}
		fmt.Fprintf(&b, " %s |\n", Verdict(rs))
	}
	b.WriteString("\n| Model | Pass | Fail | Skip | Spend |\n| --- | --- | --- | --- | --- |\n")
	for _, m := range models {
		c := count[m]
		fmt.Fprintf(&b, "| %s | %d | %d | %d | $%.4f |\n", m, c[0], c[1], c[2], spend[m])
	}
	b.WriteString("\n## Findings per cell\n\n")
	for _, n := range cells {
		for _, m := range models {
			r, ok := byCell[n][m]
			if !ok || len(r.Findings) == 0 && r.Reason == "" {
				continue
			}
			notes := []string{}
			if r.Reason != "" {
				notes = append(notes, strings.TrimSpace(r.Reason))
			}
			for _, f := range r.Findings {
				notes = append(notes, f.Check+": "+strings.ReplaceAll(f.Detail, "|", "\\|"))
			}
			fmt.Fprintf(&b, "- `%s` on `%s`: %s\n", n, m, strings.Join(notes, "; "))
		}
	}
	return b.String()
}
