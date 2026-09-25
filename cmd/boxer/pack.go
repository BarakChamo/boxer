package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/BarakChamo/boxer/internal/box"
)

// packCmd manages the prepared guests someone named: `boxer pack save base` after installing a
// toolchain, `boxer pack use base` to start a sandbox from it. The automatic packs behind
// `boxer up` are a cache boxer manages; these are artifacts a person manages, so gc never touches
// them and every failure here is reported rather than swallowed.
func packCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: boxer pack save <name> | ls [--json] | use <name> | rm <name>")
		return 2
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "ls":
		return packLs(rest, stdout, stderr)
	case "save", "use", "rm":
	default:
		fmt.Fprintf(stderr, "boxer pack: unknown subcommand %q; want save, ls, use or rm\n", sub)
		return 2
	}
	name, rest := firstOperand(rest)
	if name == "" {
		fmt.Fprintf(stderr, "usage: boxer pack %s <name>\n", sub)
		return 2
	}
	// A name becomes a path segment, so it is checked before anything else happens with it.
	if _, err := box.NamedPackPath(name); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fs, harness, id := identity("pack "+sub, rest)
	fs.SetOutput(stderr)
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	if sub == "rm" {
		p, _ := box.NamedPackPath(name)
		if err := os.Remove(p); err != nil {
			fmt.Fprintf(stderr, "boxer: no saved pack named %q\n", name)
			return 1
		}
		fmt.Fprintf(stdout, "boxer: removed pack %s\n", name)
		return 0
	}
	e, err := box.Resolve("", *harness, *id)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	e.Stderr = stderr
	switch sub {
	case "save":
		path, err := e.SavePack(name)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		size := int64(0)
		if st, serr := os.Stat(path); serr == nil {
			size = st.Size()
		}
		fmt.Fprintf(stdout, "boxer: saved pack %s (%s) — `boxer pack use %s` starts a sandbox from it\n",
			name, box.HumanBytes(size), name)
		return 0
	default: // use
		e.FromPack = name
		if _, err := e.Ensure(true, true); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "boxer: %s recreated from pack %s\n", e.Scope.Key, name)
		return 0
	}
}

func packLs(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pack ls", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print a JSON array")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rows := box.NamedPacks()
	if rows == nil {
		rows = []box.NamedPack{}
	}
	if emit(stdout, rows, *asJSON) {
		return 0
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "boxer: no saved packs; `boxer pack save <name>` snapshots this scope's sandbox")
		return 0
	}
	for _, p := range rows {
		fmt.Fprintf(stdout, "%-24s %10s  saved %s\n", p.Name, box.HumanBytes(p.Bytes), p.SavedAt.Format(time.RFC3339))
	}
	return 0
}
