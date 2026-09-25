package scope

import "testing"

// The slug is derived, not stored, so it has to be stable for a key and never mangle a name that
// is not one.
func TestSlugIsDerivedStableAndSafe(t *testing.T) {
	const key = "sb-7e1852e4a3c3"
	first := Slug(key)
	if first != Slug(key) {
		t.Fatal("a slug must be a function of the key alone")
	}
	if first == key || len(first) < 5 {
		t.Fatalf("slug: %q", first)
	}
	if Slug("sb-7e1852e4a3c3-f2") != first+"-f2" {
		t.Fatalf("a fork child reads as its parent plus the suffix: %q", Slug(key+"-f2"))
	}
	for _, other := range []string{"someone-elses-vm", "sb-", "", "sb-zzzz"} {
		if got := Slug(other); got != other {
			t.Fatalf("a name that is not a scope key must come back unchanged: %q → %q", other, got)
		}
	}
	// Different keys should usually differ; two neighbouring ones certainly should.
	if Slug("sb-000000000000") == Slug("sb-ffff00000000") {
		t.Fatal("slugs must spread across the key space")
	}
}
