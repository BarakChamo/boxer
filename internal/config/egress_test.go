package config

import (
	"math/big"
	"math/rand/v2"
	"net/netip"
	"slices"
	"strings"
	"testing"
)

func TestSplitAllowClassifies(t *testing.T) {
	e, err := SplitAllow([]string{
		"registry.npmjs.org", "API-Foo.example.com.", "10.1.2.3", "10.0.0.0/8", "fd00::1", "fd00::/64",
		"192.168.1.10-192.168.1.13", " pypi.org ", "registry.npmjs.org", "10.0.0.0/8",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"registry.npmjs.org", "api-foo.example.com", "pypi.org"}; !slices.Equal(e.Hosts, want) {
		t.Fatalf("hosts %v, want %v", e.Hosts, want)
	}
	if want := []string{"10.1.2.3/32", "10.0.0.0/8", "fd00::1/128", "fd00::/64", "192.168.1.10/31", "192.168.1.12/31"}; !slices.Equal(e.CIDRs, want) {
		t.Fatalf("cidrs %v, want %v", e.CIDRs, want)
	}
}

// Everything smolvm cannot enforce is refused at load, with a message that says why.
func TestSplitAllowRefuses(t *testing.T) {
	for entry, want := range map[string]string{
		"*.npmjs.org":            "wildcards are not supported",
		"registry*.npmjs.org":    "wildcards are not supported",
		"*":                      "wildcards are not supported",
		"registry.npmjs.org:443": "ports are not supported",
		"example.com:8000-8100":  "ports are not supported",
		"10.0.0.1:443":           "ports are not supported",
		"[fd00::1]:443":          "ports are not supported",
		"10.0.0.1/8":             "did you mean 10.0.0.0/8",
		"10.0.0.9-10.0.0.1":      "ends before it starts",
		"10.0.0.1-fd00::1":       "same family",
		"10.0.0.1-example.com":   "same family",
		"":                       "empty entry",
		"-bad.example.com":       "is not a hostname",
		"exa mple.com":           "is not a hostname",
		"http://example.com":     "is not a hostname",
	} {
		if _, err := SplitAllow([]string{entry}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want %q", entry, err, want)
		}
	}
	// A hostname with hyphens is a hostname, not a range.
	if e, err := SplitAllow([]string{"my-registry.example-corp.com"}); err != nil || len(e.Hosts) != 1 {
		t.Fatalf("%v %v", e, err)
	}
}

// A range becomes CIDR blocks that cover exactly it: every address inside, nothing outside, and no
// two blocks overlapping. Checked by counting, over ranges chosen to hit every alignment case.
func TestRangeCIDRsCoverExactly(t *testing.T) {
	for _, r := range [][2]string{
		{"10.0.0.0", "10.0.0.0"}, {"10.0.0.0", "10.0.0.255"}, {"10.0.0.1", "10.0.0.254"},
		{"10.0.0.7", "10.0.1.9"}, {"0.0.0.0", "255.255.255.255"}, {"172.16.5.3", "172.31.200.1"},
		{"255.255.255.254", "255.255.255.255"}, {"fd00::1", "fd00::1:3"}, {"::", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"},
	} {
		first, last := netip.MustParseAddr(r[0]), netip.MustParseAddr(r[1])
		blocks := rangeCIDRs(first, last)
		total := new(big.Int)
		next := first
		for i, c := range blocks {
			p := netip.MustParsePrefix(c)
			if p.Addr() != next {
				t.Fatalf("%v: block %d %s does not start at %s", r, i, c, next)
			}
			total.Add(total, new(big.Int).Lsh(big.NewInt(1), uint(p.Addr().BitLen()-p.Bits())))
			end := lastAddr(p)
			if end.Compare(last) > 0 {
				t.Fatalf("%v: block %s passes the end", r, c)
			}
			if i == len(blocks)-1 {
				if end != last {
					t.Fatalf("%v: last block ends at %s", r, end)
				}
			} else {
				next = end.Next()
			}
		}
		want := new(big.Int).Sub(new(big.Int).SetBytes(last.AsSlice()), new(big.Int).SetBytes(first.AsSlice()))
		want.Add(want, big.NewInt(1))
		if total.Cmp(want) != 0 {
			t.Fatalf("%v: blocks cover %s addresses, want %s", r, total, want)
		}
		if max := 2 * first.BitLen(); len(blocks) > max {
			t.Fatalf("%v: %d blocks, more than %d", r, len(blocks), max)
		}
	}
	if got := rangeCIDRs(netip.MustParseAddr("0.0.0.0"), netip.MustParseAddr("255.255.255.255")); !slices.Equal(got, []string{"0.0.0.0/0"}) {
		t.Fatalf("the whole space is one block: %v", got)
	}
}

// A wildcard in boxer.toml fails at load, which is before anything is created.
func TestWildcardFailsTheConfig(t *testing.T) {
	if _, err := load(t, "[network]\nallow_hosts = [\"*.npmjs.org\"]\n"); err == nil || !strings.Contains(err.Error(), "wildcards") {
		t.Fatalf("got %v", err)
	}
	if _, err := load(t, "[network]\nallow_hosts = [\"10.0.0.0/8\", \"10.1.0.1-10.1.0.9\", \"example.com\"]\n"); err != nil {
		t.Fatal(err)
	}
}

// The same property over random IPv4 ranges: blocks are contiguous, aligned, and end exactly at
// the last address.
func TestRangeCIDRsRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 2000; i++ {
		a, b := r.Uint32(), r.Uint32()
		if i%2 == 0 {
			b = a + r.Uint32N(5000)
		}
		if b < a {
			a, b = b, a
		}
		first, last := netip.AddrFrom4(u32(a)), netip.AddrFrom4(u32(b))
		next := first
		blocks := rangeCIDRs(first, last)
		for j, c := range blocks {
			p := netip.MustParsePrefix(c)
			if p.Addr() != next || p.Masked() != p {
				t.Fatalf("%s-%s: block %s misaligned or not contiguous", first, last, c)
			}
			if j < len(blocks)-1 {
				next = lastAddr(p).Next()
			} else if lastAddr(p) != last {
				t.Fatalf("%s-%s: ends at %s", first, last, lastAddr(p))
			}
		}
	}
}

func u32(v uint32) [4]byte { return [4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)} }

func TestMalformedAddressesAreRefused(t *testing.T) {
	for _, bad := range []string{"10.0.0", "010.0.0.1", "fe80::1%en0", "1.2.3.4.5"} {
		if _, err := SplitAllow([]string{bad}); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}
