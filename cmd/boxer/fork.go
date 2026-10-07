package main

import (
	"fmt"
	"io"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/vm"
)

// forkCmd branches this scope's sandbox into copy-on-write children: the same guest, its RAM and
// its processes already warm, for work that wants many warm workers over one worktree.
//
// Children share the parent's live worktree. smolvm refuses to branch a machine whose mount is
// staged, so there is no per-child copy to offer and two children writing one file race exactly
// as two processes on the host would. That is the boundary of what a fork is good for.
//
// Preparing is explicit because it restarts the sandbox, which ends whatever is running in it.
func forkCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "ls":
			return forkLs(args[1:], stdout, stderr)
		case "rm":
			return forkRm(args[1:], stdout, stderr)
		}
	}
	fs, harness, id := identity("fork", args)
	count := fs.Int("count", 1, "how many children to branch")
	prepare := fs.Bool("prepare", false, "restart the sandbox as a branch source first")
	asJSON := fs.Bool("json", false, "print the children as JSON")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	e, err := box.Resolve("", *harness, *id)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	e.Stderr = stderr
	if *prepare {
		if err := e.PrepareFork(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		// Narration goes to stderr: stdout is the list of children, and --json means only that.
		fmt.Fprintf(stderr, "boxer: %s is a branch source; its children share this worktree\n", e.Scope.Key)
	}
	names, err := e.Fork(*count)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	// --count is how many children this scope should have, so asking for two when one is already
	// there makes one. Nothing new is not an error, and it must not be an empty JSON `null`
	// either: a caller that indexes the array would crash on it.
	if names == nil {
		names = []string{}
	}
	if emit(stdout, names, *asJSON) {
		return 0
	}
	for _, n := range names {
		fmt.Fprintf(stdout, "%s\n", n)
	}
	if len(names) == 0 {
		fmt.Fprintf(stderr, "boxer: %s already has that many forks; `boxer fork ls` lists them\n", e.Scope.Key)
		return 0
	}
	fmt.Fprintf(stderr, "boxer: run in one with `boxer run --scope %s -c '<command>'`; `boxer fork rm --all` reclaims them\n", names[0])
	return 0
}

func forkLs(args []string, stdout, stderr io.Writer) int {
	fs, harness, id := identity("fork ls", args)
	asJSON := fs.Bool("json", false, "print a JSON array")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	e, err := box.Resolve("", *harness, *id)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	kids, err := e.Forks()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rows := []machineJSON{}
	for _, m := range kids {
		rows = append(rows, machineRow(m))
	}
	if emit(stdout, rows, *asJSON) {
		return 0
	}
	if len(rows) == 0 {
		fmt.Fprintf(stdout, "boxer: %s has no forks (boxer fork --prepare)\n", e.Scope.Key)
		return 0
	}
	for _, r := range rows {
		fmt.Fprintf(stdout, "%-24s %s\n", r.Scope, r.State)
	}
	return 0
}

func forkRm(args []string, stdout, stderr io.Writer) int {
	fs, harness, id := identity("fork rm", args)
	all := fs.Bool("all", false, "remove every fork of this scope")
	fs.SetOutput(stderr)
	name, rest := firstOperand(args)
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	client := vm.Host(hostBackend())
	var names []string
	switch {
	case *all:
		e, err := box.Resolve("", *harness, *id)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		kids, err := e.Forks()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, m := range kids {
			names = append(names, m.Name)
		}
	case name != "":
		names = []string{box.ResolveName(client, name)}
	default:
		fmt.Fprintln(stderr, "usage: boxer fork rm <name> | --all")
		return 2
	}
	code := 0
	for _, n := range names {
		if !box.IsForkChild(n) {
			fmt.Fprintf(stderr, "boxer: %q is not a fork child; use `boxer down --scope %s` for a sandbox\n", n, n)
			code = 2
			continue
		}
		if owned, err := vm.OwnsName(client, n); err != nil || !owned {
			if err == nil {
				err = vm.NotOwned(client, n)
			}
			fmt.Fprintln(stderr, err)
			code = 1
			continue
		}
		if err := client.Delete(n); err != nil {
			fmt.Fprintln(stderr, err)
			code = 1
			continue
		}
		// Everything a deleted sandbox leaves, as for every other delete.
		vm.ForgetOwned(client, n)
		box.ForgetScope(n)
		fmt.Fprintf(stdout, "boxer: %s removed\n", n)
	}
	if len(names) == 0 {
		fmt.Fprintln(stdout, "boxer: no forks to remove")
	}
	return code
}
