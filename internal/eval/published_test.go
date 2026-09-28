package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The published numbers — the site's results page and docs/status.md — are written by hand from
// generated evidence, and a hand-written number drifts: status.md was once a whole run behind the
// scorecard it summarised. This test reads the evidence and fails when a published number no
// longer matches it. Re-running an evaluation without updating the page now fails the build,
// which is the point.
func TestPublishedNumbersMatchTheEvidence(t *testing.T) {
	root := filepath.Join("..", "..")
	read := func(rel string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	results := read("site/content/docs/evals/results.mdx")
	status := read("docs/status.md")

	// The matrix: the headline percentage and cells at 100%, from each generated report.
	headline := regexp.MustCompile(`\*\*([\d.]+)% overall\*\*.*?(\d+) of (\d+) cells scored 100%`)
	for _, rep := range []string{"docs/eval-matrix-report.md", "docs/eval-matrix-urls-report.md"} {
		m := headline.FindStringSubmatch(read(rep))
		if m == nil {
			t.Fatalf("%s has no headline", rep)
		}
		pct, cells := m[1]+"%", m[2]+" of "+m[3]
		if !strings.Contains(results, "**"+pct+"**") || !strings.Contains(results, cells) {
			t.Errorf("results.mdx does not carry %s's %s, %s at 100%%", rep, pct, cells)
		}
		if !strings.Contains(status, pct) {
			t.Errorf("status.md does not carry %s's %s", rep, pct)
		}
	}

	// Adherence: each scenario's pass count, from every row recorded for it.
	type row struct {
		Cell struct {
			Scenario string
		}
		Status string
	}
	tally := map[string][2]int{}
	f, err := os.Open(filepath.Join(root, "docs", "eval-adherence.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var r row
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		v := tally[r.Cell.Scenario]
		v[1]++
		if r.Status == "pass" {
			v[0]++
		}
		tally[r.Cell.Scenario] = v
	}
	_ = f.Close()
	for _, scenario := range []string{"prep", "server"} {
		v := tally[scenario]
		if v[1] == 0 {
			t.Errorf("no %s rows in eval-adherence.jsonl", scenario)
			continue
		}
		if want := fmt.Sprintf("**%d of %d**", v[0], v[1]); !strings.Contains(results, want) {
			t.Errorf("results.mdx does not carry the %s scenario's %s", scenario, want)
		}
	}

	// Smoke: the latest recorded run per backend on each host, against the per-backend tables.
	type smoke struct {
		Backend, Host           string
		Passed, Failed, Skipped int
	}
	latest := map[string]smoke{}
	b, err := os.ReadFile(filepath.Join(root, "docs", "smoke-results.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var s smoke
		if json.Unmarshal([]byte(line), &s) == nil {
			latest[s.Host+"/"+s.Backend] = s
		}
	}
	if len(latest) == 0 {
		t.Fatal("no smoke runs recorded")
	}
	name := map[string]string{"container": "Apple `container`"}
	for key, s := range latest {
		if s.Failed != 0 {
			t.Errorf("the latest recorded %s run failed %d cells; the page cannot say it passed", key, s.Failed)
		}
		label := s.Backend
		if n, ok := name[s.Backend]; ok {
			label = n
		}
		if s.Host == "linux" {
			label += " on Linux"
		}
		cell := fmt.Sprintf("| %d | %d |", s.Passed, s.Skipped)
		if s.Skipped == 0 {
			cell = fmt.Sprintf("| %d of %d | 0 |", s.Passed, s.Passed)
		}
		if !strings.Contains(results, "| "+label+" "+cell) {
			t.Errorf("results.mdx has no row %q for the latest %s run", "| "+label+" "+cell, key)
		}
	}
}

// The benchmarks page is held to its raw files the same way: the backend table must carry the
// median of each cell in bench/backends.jsonl, and the [urls] table the medians in
// bench/urls.jsonl. Re-running a benchmark without updating the page fails here.
func TestPublishedBenchmarksMatchTheirData(t *testing.T) {
	root := filepath.Join("..", "..")
	page, err := os.ReadFile(filepath.Join(root, "site", "content", "docs", "evals", "benchmarks.mdx"))
	if err != nil {
		t.Fatal(err)
	}
	median := func(xs []float64) float64 {
		slices.Sort(xs)
		if n := len(xs); n%2 == 1 {
			return xs[n/2]
		} else {
			return (xs[n/2-1] + xs[n/2]) / 2
		}
	}
	rows := func(name string) []map[string]any {
		b, err := os.ReadFile(filepath.Join(root, "bench", name))
		if err != nil {
			t.Fatal(err)
		}
		var out []map[string]any
		for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			var m map[string]any
			if err := json.Unmarshal([]byte(l), &m); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			out = append(out, m)
		}
		return out
	}

	cells := map[string]map[float64][]float64{}
	for _, r := range rows("backends.jsonl") {
		c := r["contender"].(string)
		if cells[c] == nil {
			cells[c] = map[float64][]float64{}
		}
		cells[c][r["n"].(float64)] = append(cells[c][r["n"].(float64)], r["seconds"].(float64))
	}
	label := map[string]string{"docker": "docker", "container": "Apple `container`", "smolvm": "smolvm", "podman": "podman"}
	for c, byN := range cells {
		re := regexp.MustCompile(`\| ` + regexp.QuoteMeta(label[c]) + ` \| [^|]+ \| \**([0-9.]+) s\** \| \**([0-9.]+) s\** \|`)
		m := re.FindStringSubmatch(string(page))
		if m == nil {
			t.Errorf("benchmarks.mdx has no backend row for %s", c)
			continue
		}
		for i, n := range []float64{1, 3} {
			if want := fmt.Sprintf("%.1f", median(byN[n])); m[i+1] != want {
				t.Errorf("benchmarks.mdx: %s at %v worktree(s) is %s s, data median %s s", c, n, m[i+1], want)
			}
		}
	}

	urls := map[string][]float64{}
	for _, r := range rows("urls.jsonl") {
		k := r["urls"].(string) + "/" + r["call"].(string)
		urls[k] = append(urls[k], r["ms"].(float64))
	}
	for _, arm := range []string{"off", "on"} {
		want := fmt.Sprintf("| `[urls]` %s | %.1f ms | %.1f ms |", arm, median(urls[arm+"/up"]), median(urls[arm+"/run"]))
		if !strings.Contains(string(page), want) {
			t.Errorf("benchmarks.mdx does not carry %q", want)
		}
	}
}
