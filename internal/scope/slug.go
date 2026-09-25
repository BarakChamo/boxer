package scope

import (
	"encoding/hex"
	"strings"
)

// A scope key is a hash because it has to be a legal machine name whatever the worktree path is.
// That makes it unreadable, and it is read constantly: in listings, in errors, in agent
// transcripts, in the argument to --scope. The slug is the same twelve hex characters said out
// loud — derived, never stored, so two boxers agree without coordinating, and the same worktree
// always gets the same pair of words.
//
// It is a label, not an identifier: collisions are possible and harmless, because every place
// that prints a slug prints the key beside it and every place that accepts one also accepts the
// key.

// slugAdjectives and slugNouns are 64 each, so a slug consumes 12 bits of the key and there are
// 4096 pairs. Short, ordinary, and unambiguous when read aloud.
var slugAdjectives = [...]string{
	"amber", "ancient", "arctic", "autumn", "bold", "brave", "brass", "bright",
	"calm", "clever", "coastal", "copper", "crimson", "curious", "dapper", "deep",
	"eager", "early", "electric", "emerald", "fair", "fleet", "fond", "frosty",
	"gentle", "glad", "golden", "grand", "hidden", "honest", "humble", "iron",
	"jolly", "keen", "kind", "late", "lively", "lucid", "lucky", "merry",
	"mighty", "mild", "noble", "nimble", "olive", "patient", "plain", "polar",
	"quick", "quiet", "rapid", "royal", "rugged", "scarlet", "silent", "silver",
	"smooth", "solid", "steady", "swift", "tidy", "vivid", "warm", "wise",
}

var slugNouns = [...]string{
	"albatross", "anchor", "aspen", "badger", "beacon", "bison", "cedar", "comet",
	"coral", "crab", "crane", "dolphin", "eagle", "elk", "ember", "falcon",
	"fern", "finch", "fjord", "fox", "gale", "gannet", "harbor", "hawk",
	"heron", "ibis", "isle", "jay", "kestrel", "lantern", "lark", "lemur",
	"lynx", "maple", "marlin", "meadow", "mesa", "moth", "narwhal", "oak",
	"orca", "osprey", "otter", "owl", "panther", "petrel", "pine", "puffin",
	"quail", "raven", "reef", "ridge", "salmon", "sparrow", "spruce", "stoat",
	"summit", "swift", "tern", "thrush", "tundra", "walrus", "willow", "wren",
}

// Slug renders a scope key as two words. A name that is not a scope key comes back unchanged, so
// a caller can pass any machine name through it.
func Slug(key string) string {
	hexPart := strings.TrimPrefix(key, "sb-")
	// A fork child is "<key>-f<n>": slug the parent and keep the suffix, so a child reads as one.
	suffix := ""
	if i := strings.Index(hexPart, "-"); i >= 0 {
		suffix, hexPart = hexPart[i:], hexPart[:i]
	}
	if !strings.HasPrefix(key, "sb-") || len(hexPart) < 4 {
		return key
	}
	b, err := hex.DecodeString(hexPart[:4])
	if err != nil {
		return key
	}
	n := int(b[0])<<8 | int(b[1])
	return slugAdjectives[(n>>6)&63] + "-" + slugNouns[n&63] + suffix
}

// Slug is the scope's own two-word name.
func (s Scope) Slug() string { return Slug(s.Key) }

// Names reports the scope as "swift-crab (sb-7e1852e4a3c3)": the readable name first, because
// that is what a person matches on, and the key beside it, because that is what is unique.
func (s Scope) Names() string { return Slug(s.Key) + " (" + s.Key + ")" }
