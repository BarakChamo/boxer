// Command fanout runs one command in several worktrees at once, each in its own boxer sandbox, and
// reports every exit code. It is the core of what an orchestrator does with pkg/boxer.
//
//	go run ./examples/go-embed -- 'npm test' ~/code/app-wt1 ~/code/app-wt2
//	go run ./examples/go-embed -down -- 'npm test' ~/code/app-wt1 ~/code/app-wt2
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/BarakChamo/boxer/pkg/boxer"
)

type result struct {
	dir  string
	code int
	out  string
	err  error
}

func main() {
	down := flag.Bool("down", false, "delete each sandbox after its run")
	flag.Parse()
	if flag.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: fanout [-down] -- '<shell line>' <worktree>...")
		os.Exit(2)
	}
	line, dirs := flag.Arg(0), flag.Args()[1:]

	results := make([]result, len(dirs))
	var wg sync.WaitGroup
	for i, dir := range dirs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = run(dir, line, *down)
		}()
	}
	wg.Wait()

	failed := 0
	for _, r := range results {
		switch {
		case r.err != nil:
			failed++
			fmt.Printf("%-40s error: %v\n", r.dir, r.err)
		case r.code != 0:
			failed++
			fmt.Printf("%-40s exit %d\n%s", r.dir, r.code, indent(r.out))
		default:
			fmt.Printf("%-40s ok\n", r.dir)
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}

func run(dir, line string, down bool) result {
	r := result{dir: dir}
	b, err := boxer.Open(dir, boxer.Options{Harness: "fanout"})
	if err != nil {
		// A refusal carries a stable Cause and a Fix written for a person.
		var be *boxer.Error
		if errors.As(err, &be) {
			err = fmt.Errorf("%s (%s); fix: %s", be.Reason, be.Cause, be.Fix)
		}
		r.err = err
		return r
	}
	// Idempotent: creates the sandbox the first time, does nothing on a running one.
	if _, err := b.Ensure(true, false); err != nil {
		r.err = err
		return r
	}
	var out bytes.Buffer
	r.code, r.err = b.Run([]string{"sh", "-c", line}, boxer.RunOpts{
		Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out,
	})
	r.out = out.String()
	if down {
		if err := b.Down(); err != nil && r.err == nil {
			r.err = err
		}
	}
	return r
}

func indent(s string) string {
	if s == "" {
		return ""
	}
	return "    " + strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "\n    ") + "\n"
}
