// Command boxer-eval runs the harness and orchestrator evaluation matrix.
//
//	boxer-eval --tier t1                      # deterministic: real harness, fake model
//	boxer-eval --tier t2 --harness claude-code # live: the harness's own login
//	boxer-eval --tier t1 --cell claude-code/tool --keep
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BarakChamo/boxer/internal/eval"
)

func main() {
	tier := flag.String("tier", "t1", "t1 (fake model) or t2 (live)")
	harness := flag.String("harness", "", "comma-separated driver names; default all")
	cell := flag.String("cell", "", "substring filter on cell names")
	out := flag.String("out", "", "write the Markdown report here")
	keep := flag.Bool("keep", false, "keep each cell's scratch directory")
	list := flag.Bool("list", false, "list cells and exit")
	lockRun := flag.Bool("lock-run", false, "take the host smolvm lock, then run the command after -- (used by evals/smoke.sh)")
	flag.Parse()

	if *lockRun {
		unlock, err := eval.HostLock(os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "lock:", err)
			os.Exit(2)
		}
		cmd := exec.Command(flag.Arg(0), flag.Args()[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		cmd.Env = append(os.Environ(), "BOXER_EVAL_LOCKED=1")
		err = cmd.Run()
		unlock()
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		} else if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}

	boxerBin, err := exec.LookPath("boxer")
	if err != nil {
		if p, e := filepath.Abs("bin/boxer"); e == nil {
			if _, e := os.Stat(p); e == nil {
				boxerBin = p
				err = nil
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "boxer binary not found on PATH or at bin/boxer")
		os.Exit(2)
	}

	drivers := eval.Drivers()
	if *harness != "" {
		want := map[string]bool{}
		for _, h := range strings.Split(*harness, ",") {
			want[strings.TrimSpace(h)] = true
		}
		var picked []eval.Driver
		for _, d := range drivers {
			if want[d.Name()] {
				picked = append(picked, d)
			}
		}
		drivers = picked
	}
	only := func(c eval.Cell) bool { return *cell == "" || strings.Contains(c.Name(), *cell) }
	if *list {
		for _, d := range drivers {
			for _, c := range d.Cells(*tier) {
				if only(c) {
					fmt.Println(c.Name())
				}
			}
		}
		return
	}
	results := eval.Run(drivers, *tier, boxerBin, only, *keep, os.Stderr)
	report := eval.Report(results, *tier)
	if *out != "" {
		os.WriteFile(*out, []byte(report), 0o644)
	}
	fmt.Println(report)
	for _, r := range results {
		if r.Status == "fail" {
			os.Exit(1)
		}
	}
}
