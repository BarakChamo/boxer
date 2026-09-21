// The report and the archive: one run's scorecard, and the series of runs it belongs to.
//
// A single latest-report file answers "is it working now" and nothing else. What a published eval
// has to answer is "is it getting better, and what changed when it got worse", which needs the runs
// kept side by side, each carrying the conditions it was made under.
package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// RunMeta is the provenance of one run. A score without the conditions that produced it is not
// evidence, and a published series of them is worth nothing if the reader cannot tell which runs
// are comparable: the model, the parallelism, the boxer under test and the host all move the
// numbers.
type RunMeta struct {
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
	Boxer    string    `json:"boxer"`    // version as the binary reports it
	Commit   string    `json:"commit"`   // the tree the run was made from
	Dirty    bool      `json:"dirty"`    // whether that tree had uncommitted changes
	Smolvm   string    `json:"smolvm"`   // the hypervisor underneath
	Model    string    `json:"model"`    // every cell runs on one model, on purpose
	Image    string    `json:"image"`    // the guest image the workload runs in
	Parallel int       `json:"parallel"` // how many cells at once
	Host     string    `json:"host"`     // os/arch, cpus, memory
	Cells    int       `json:"cells"`
	Full     int       `json:"full"` // cells at 100%
	Skipped  int       `json:"skipped"`
	Score    float64   `json:"score"`
	Spend    float64   `json:"spend"`
}

// CollectRunMeta reads what it can and leaves the rest empty rather than guessing.
func CollectRunMeta(boxerBin string, parallel int, started time.Time) RunMeta {
	one := func(name string, args ...string) string {
		out, err := exec.Command(name, args...).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	m := RunMeta{
		Started:  started,
		Boxer:    one(boxerBin, "--version"),
		Commit:   one("git", "rev-parse", "--short", "HEAD"),
		Dirty:    one("git", "status", "--porcelain") != "",
		Smolvm:   one("smolvm", "--version"),
		Model:    LiveModel("claude"),
		Image:    matrixImage,
		Parallel: parallel,
		Host: fmt.Sprintf("%s/%s, %d cpus, %s", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(),
			strings.TrimSpace(one("sh", "-c", "sysctl -n hw.memsize 2>/dev/null | awk '{printf \"%.0f GB\", $1/1073741824}'"))),
	}
	return m
}

// MatrixReport is the scorecard. It is organised by level rather than by task, because the question
// the tier answers is "which ways into the sandbox support development", not "which tasks pass" —
// and it reports a percentage per cell rather than a verdict, because a cell that renders the page
// and leaves the change behind but cannot show which layer carried the work is not the same result
// as one whose sandbox never started.
func MatrixReport(rs []MatrixResult, parallel int) string {
	return MatrixReportWith(rs, parallel, RunMeta{})
}

// MatrixReportWith is the report with the run's provenance at the top.
func MatrixReportWith(rs []MatrixResult, parallel int, meta RunMeta) string {
	var b strings.Builder
	var got, total int
	full, ran, skipped := 0, 0, 0
	var spend float64
	for _, r := range rs {
		if r.Status == "skip" {
			skipped++
			continue
		}
		ran++
		g, t := r.Checks.score()
		got, total = got+g, total+t
		if r.Score == 100 {
			full++
		}
		spend += r.Spend
	}
	overall := 0.0
	if total > 0 {
		overall = 100 * float64(got) / float64(total)
	}
	fmt.Fprintf(&b, "# boxer eval report — tier matrix — %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "The same Next.js development workload across integration levels, harnesses and\n")
	fmt.Fprintf(&b, "orchestrators: %d cells, %d at a time, each in its own git worktree with its own sandbox and\n", ran, parallel)
	fmt.Fprintf(&b, "its own automatic host port. Every cell is scored on each claim it makes separately —\n")
	fmt.Fprintf(&b, "provisioning, reaching the guest, rendering the page, leaving the change behind, and showing\n")
	fmt.Fprintf(&b, "that the integration level itself carried the work — so a shortfall names which claim failed.\n\n")
	fmt.Fprintf(&b, "**%.1f%% overall** (%d of %d weighted checks). %d of %d cells scored 100%%, %d skipped, $%.2f.\n\n",
		overall, got, total, full, ran, skipped, spend)
	if meta.Boxer != "" || meta.Commit != "" {
		dirty := ""
		if meta.Dirty {
			dirty = " (with uncommitted changes)"
		}
		fmt.Fprintf(&b, "## How this run was made\n\n")
		fmt.Fprintf(&b, "| | |\n| --- | --- |\n")
		fmt.Fprintf(&b, "| boxer | %s, commit `%s`%s |\n", dash(meta.Boxer), dash(meta.Commit), dirty)
		fmt.Fprintf(&b, "| smolvm | %s |\n", dash(meta.Smolvm))
		fmt.Fprintf(&b, "| model | `%s` — every cell, so the integration level is the only variable |\n", dash(meta.Model))
		fmt.Fprintf(&b, "| guest image | `%s` |\n", dash(meta.Image))
		fmt.Fprintf(&b, "| concurrency | %d cells at a time |\n", meta.Parallel)
		fmt.Fprintf(&b, "| host | %s |\n", dash(meta.Host))
		fmt.Fprintf(&b, "| started | %s |\n", meta.Started.Format(time.RFC3339))
		if !meta.Finished.IsZero() {
			fmt.Fprintf(&b, "| took | %s |\n", meta.Finished.Sub(meta.Started).Round(time.Second))
		}
		fmt.Fprintln(&b)
	}
	for _, r := range rs {
		if r.Status == "skip" && outOfBudget(r.Skipped) {
			fmt.Fprintf(&b, "> This run stopped early: **%s**. The cells below it were not attempted, and the\n", r.Skipped)
			fmt.Fprintf(&b, "> score above covers only the cells that ran. Raise the gateway budget and run it again.\n\n")
			break
		}
	}

	// By level first: the verdict this tier exists for.
	type agg struct{ got, total, full, cells int }
	byLevel, byHarness := map[string]*agg{}, map[string]*agg{}
	var levels, harnesses []string
	for _, r := range rs {
		if r.Status == "skip" {
			continue
		}
		h := strings.SplitN(r.Config, "/", 2)[0]
		for _, e := range []struct {
			m   map[string]*agg
			k   string
			ord *[]string
		}{{byLevel, r.Level, &levels}, {byHarness, h, &harnesses}} {
			a := e.m[e.k]
			if a == nil {
				a = &agg{}
				e.m[e.k] = a
				*e.ord = append(*e.ord, e.k)
			}
			g, t := r.Checks.score()
			a.got, a.total, a.cells = a.got+g, a.total+t, a.cells+1
			if r.Score == 100 {
				a.full++
			}
		}
	}
	section := func(title, col string, order []string, m map[string]*agg) {
		fmt.Fprintf(&b, "## By %s\n\n| %s | score | cells at 100%% |\n| --- | --- | --- |\n", title, col)
		for _, k := range order {
			a := m[k]
			pct := 0.0
			if a.total > 0 {
				pct = 100 * float64(a.got) / float64(a.total)
			}
			fmt.Fprintf(&b, "| %s | %.1f%% | %d/%d |\n", k, pct, a.full, a.cells)
		}
		fmt.Fprintln(&b)
	}
	sort.Strings(levels)
	sort.Strings(harnesses)
	section("level", "level", levels, byLevel)
	section("harness", "harness", harnesses, byHarness)

	fmt.Fprintf(&b, "## Every cell\n\n| cell | level | score | guest | host port | turns | time | what fell short |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rs {
		if r.Status == "skip" {
			fmt.Fprintf(&b, "| %s/%s | %s | skipped | — | — | — | — | %s |\n", r.Config, r.Task, r.Level, r.Skipped)
			continue
		}
		fmt.Fprintf(&b, "| %s/%s | %s | %.0f%% | %s | %s | %d | %s | %s |\n", r.Config, r.Task, r.Level, r.Score,
			dash(r.Guest), dash(r.HostPort), r.Turns, r.Total.Round(time.Second), dash(strings.Join(r.Checks.failed(), "; ")))
	}

	// The scorecard proper: which claim failed, and where.
	fmt.Fprintf(&b, "\n## The scorecard\n\nEvery claim, and how many cells made it good.\n\n")
	type claim struct{ pass, of int }
	claims, order := map[string]*claim{}, []string{}
	for _, r := range rs {
		for _, c := range r.Checks {
			cl := claims[c.Name]
			if cl == nil {
				cl = &claim{}
				claims[c.Name] = cl
				order = append(order, c.Name)
			}
			cl.of++
			if c.Passed {
				cl.pass++
			}
		}
	}
	fmt.Fprintf(&b, "| claim | cells | met |\n| --- | --- | --- |\n")
	for _, n := range order {
		cl := claims[n]
		fmt.Fprintf(&b, "| %s | %d | %d (%.0f%%) |\n", n, cl.of, cl.pass, 100*float64(cl.pass)/float64(cl.of))
	}

	fmt.Fprintf(&b, "\n## What each agent did\n\n")
	for _, r := range rs {
		if r.Skipped != "" {
			continue
		}
		fmt.Fprintf(&b, "### %s — %s — %.0f%%\n\n", r.Config, r.Task, r.Score)
		fmt.Fprintf(&b, "- level `%s`, %s, %d tool calls\n", r.Level, r.Total.Round(time.Second), r.Turns)
		if len(r.Tools) > 0 {
			var names []string
			for n, c := range r.Tools {
				names = append(names, fmt.Sprintf("%s×%d", n, c))
			}
			sort.Strings(names)
			fmt.Fprintf(&b, "- tools: %s\n", strings.Join(names, ", "))
		}
		if r.Carried != "" {
			fmt.Fprintf(&b, "- the level carried it: %s\n", r.Carried)
		}
		for _, c := range r.Checks {
			if !c.Passed {
				fmt.Fprintf(&b, "- **%s**: %s\n", c.Name, c.Detail)
			}
		}
		if len(r.Changed) > 0 {
			fmt.Fprintf(&b, "- changed: %s\n", strings.Join(r.Changed, ", "))
		}
		if r.Transcript != "" {
			fmt.Fprintf(&b, "- transcript: `%s`\n", r.Transcript)
		}
		if r.Trace != "" {
			fmt.Fprintf(&b, "- trace: `%s`\n", r.Trace)
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// ArchiveRun keeps one run for good: its full report under dir, a line in the index beside every
// other run, and a JSON line for reading the series by machine.
//
// A single latest-report file answers "is it working now" and nothing else. What a published eval
// has to answer is "is it getting better, and what changed when it got worse" — which needs the
// runs kept side by side, each carrying the conditions it was made under.
func ArchiveRun(dir string, rs []MatrixResult, parallel int, meta RunMeta, report string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stamp := meta.Started.UTC().Format("2006-01-02T150405Z")
	name := stamp + ".md"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(report), 0o644); err != nil {
		return err
	}

	for _, r := range rs {
		if r.Status == "skip" {
			meta.Skipped++
			continue
		}
		meta.Cells++
		if r.Score == 100 {
			meta.Full++
		}
		meta.Spend += r.Spend
	}
	var got, total int
	for _, r := range rs {
		g, t := r.Checks.score()
		got, total = got+g, total+t
	}
	if total > 0 {
		meta.Score = 100 * float64(got) / float64(total)
	}
	meta.Finished = time.Now()

	line, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "runs.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return writeRunIndex(dir)
}

// writeRunIndex rebuilds the index from runs.jsonl, newest first, so the file is always a true
// reflection of what is archived rather than something that drifts as runs are added.
func writeRunIndex(dir string) error {
	b, err := os.ReadFile(filepath.Join(dir, "runs.jsonl"))
	if err != nil {
		return err
	}
	var runs []RunMeta
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var m RunMeta
		if json.Unmarshal([]byte(line), &m) == nil {
			runs = append(runs, m)
		}
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].Started.After(runs[j].Started) })

	var x strings.Builder
	fmt.Fprintf(&x, "# boxer eval runs — tier matrix\n\n")
	fmt.Fprintf(&x, "Every run of the matrix tier, newest first. Each row links to that run's full\n")
	fmt.Fprintf(&x, "scorecard: the score by level, by harness and by claim, and what every cell did.\n")
	fmt.Fprintf(&x, "`runs.jsonl` carries the same rows for reading the series by machine.\n\n")
	fmt.Fprintf(&x, "A run is comparable with another only when the model, the concurrency and the host\n")
	fmt.Fprintf(&x, "match, so each row carries them.\n\n")
	fmt.Fprintf(&x, "| run | score | cells at 100%% | model | boxer | spend |\n")
	fmt.Fprintf(&x, "| --- | --- | --- | --- | --- | --- |\n")
	for _, m := range runs {
		stamp := m.Started.UTC().Format("2006-01-02T150405Z")
		commit := m.Commit
		if m.Dirty {
			commit += "+"
		}
		fmt.Fprintf(&x, "| [%s](%s.md) | %.1f%% | %d/%d | `%s` | `%s` | $%.2f |\n",
			m.Started.UTC().Format("2006-01-02 15:04"), stamp, m.Score, m.Full, m.Cells, m.Model, commit, m.Spend)
	}
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(x.String()), 0o644)
}
