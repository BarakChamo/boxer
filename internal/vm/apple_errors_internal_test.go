package vm

import (
	"os"
	"path/filepath"
	"testing"
)

// Apple's CLI marks its own failures with its error codes; a guest's "Error: ..." is the guest's.
func TestAppleOwnErrorsAreNotTheGuests(t *testing.T) {
	for line, own := range map[string]bool{
		"Error: get failed: container sb-1 not found": true,
		"Error: invalidState: container is stopped":   true,
		"Error: tests failed":                         false,
		"Error: Os { code: 2, kind: NotFound }":       false,
	} {
		h := &headWriter{head: []byte(line + "\n")}
		if got := runtimeFailure(NewApple(), 1, h, map[int][]string{1: appleOwnErrors}, classifyApple) != nil; got != own {
			t.Errorf("%q: backend failure = %v, want %v", line, got, own)
		}
	}
}

// A container removed between `ps` and `inspect` is skipped, not a failure of the whole listing.
func TestDockerListSurvivesAVanishingContainer(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "docker")
	script := `#!/bin/sh
case "$1" in
ps) echo a; echo gone ;;
inspect)
  shift
  for n in "$@"; do [ "$n" = gone ] && { echo "Error: No such object: gone" >&2; exit 1; }; done
  echo '[{"Name":"/a","State":{"Status":"running"},"Config":{"Image":"alpine","Labels":{"boxer.scope":"a"}}}]' ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ms, err := NewDocker(bin).List()
	if err != nil || len(ms) != 1 || ms[0].Name != "a" {
		t.Fatalf("list: %v %+v", err, ms)
	}
}
