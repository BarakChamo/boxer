package vm

import (
	"errors"
	"fmt"
	"strings"
)

// The conditions boxer branches on, named once so no caller has to know any backend's wording.
//
// Before these existed, `internal/box` decided what to do by matching substrings of smolvm's
// stderr — `box.go` read "already exists" and "database is locked" through predicates that this
// package exported for the purpose. That works for exactly one backend. A second one says
// "No such container" and "Conflict. The container name ... is already in use", and every
// predicate silently returns false: a create that lost a race stops being recognised as a race,
// and boxer reports a hard failure for a sandbox that is fine.
//
// So the wire between a backend and the rest of boxer is a sentinel, and the substring matching
// stays inside each backend, where the strings belong.
var (
	ErrNotFound      = errors.New("machine not found")
	ErrAlreadyExists = errors.New("machine already exists")
	ErrNotRunning    = errors.New("machine not running")
	ErrBusy          = errors.New("the backend's own store is busy")
	ErrUnsupported   = errors.New("unsupported by this backend")
)

// Error is a failed backend invocation. It carries the verb, the exit code and what the backend
// said, plus Kind: the condition it means, classified once by the backend that produced it.
//
// Read Kind through errors.Is and the predicates below. Do not read Stderr to make a decision —
// the strings a backend chooses are that backend's business, and the whole point of Kind is that
// no caller outside this package has to know them.
type Error struct {
	Backend string // which backend said it; "smolvm" when unset, for older callers
	Verb    string // the subcommand: "machine", "pack", "--version"
	Code    int    // its exit code, -1 when it never ran
	Stderr  string // what it printed, stderr preferred
	Kind    error  // one of the sentinels above, or nil when it is none of them
}

func (e *Error) Error() string {
	b := e.Backend
	if b == "" {
		b = "smolvm"
	}
	return fmt.Sprintf("%s %s: %s", b, e.Verb, e.Stderr)
}

// Unwrap exposes Kind so errors.Is finds it. A nil Kind simply ends the chain.
func (e *Error) Unwrap() error { return e.Kind }

// IsNotFound reports a machine the backend does not have.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsAlreadyExists reports a create refused because the name is taken or is being taken.
func IsAlreadyExists(err error) bool { return errors.Is(err, ErrAlreadyExists) }

// IsLocked reports the backend's own store being busy: smolvm keeps machine records in SQLite and
// a concurrent `machine create` loses the race with "database is locked". It says nothing about
// the request, so the only correct response is to wait and ask again.
//
// This matters more than it looks. boxer's whole shape is one sandbox per worktree, so several
// creates arriving together is the normal case, not a corner — and the caller that mistakes this
// for a broken request does real damage: boxer used to read it as a corrupt environment pack,
// delete the pack, and pull the image from the registry instead. Four parallel provisions took
// 27s each because the first one destroyed the cache the other three were about to use.
//
// A backend with a daemon that serialises for itself never returns this, and needs no retry loop.
func IsLocked(err error) bool { return errors.Is(err, ErrBusy) }

// IsNotRunning reports an operation refused because the machine is stopped.
func IsNotRunning(err error) bool { return errors.Is(err, ErrNotRunning) }

// classifySmolvm maps smolvm's wording onto the sentinels. It is smolvm's dialect and lives with
// smolvm; another backend brings its own.
//
// The order matters only for a message that matches two patterns, which none of smolvm 1.16.1's
// do — they are distinct sentences. If that ever stops being true, the first match wins and the
// test below is what will notice.
func classifySmolvm(stderr string) error {
	switch {
	case strings.Contains(stderr, "not found"):
		return ErrNotFound
	case strings.Contains(stderr, "already exists"):
		return ErrAlreadyExists
	case strings.Contains(stderr, "database is locked"):
		return ErrBusy
	case strings.Contains(stderr, "not running"):
		return ErrNotRunning
	}
	return nil
}
