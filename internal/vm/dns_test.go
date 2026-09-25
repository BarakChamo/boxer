package vm

import (
	"net"
	"strings"
	"testing"
)

// A loopback resolver is the failure this guard exists for: a host running dnsmasq or
// systemd-resolved advertises 127.0.0.1, which inside the guest names the guest, so passing it
// through would turn slow DNS into no DNS.
func TestFirstUsableSkipsAddressesAGuestCannotReach(t *testing.T) {
	for _, c := range []struct {
		in   []string
		want string
	}{
		{[]string{"127.0.0.1", "8.8.8.8"}, "8.8.8.8"},
		{[]string{"::1", "1.1.1.1"}, "1.1.1.1"},
		{[]string{"0.0.0.0", "192.168.1.1"}, "192.168.1.1"},
		{[]string{"169.254.1.1", "10.0.0.1"}, "10.0.0.1"},
		{[]string{"not-an-ip", "198.18.0.2"}, "198.18.0.2"},
		{[]string{"127.0.0.1"}, ""},
		{nil, ""},
	} {
		if got := firstUsable(c.in); got != c.want {
			t.Errorf("firstUsable(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestScanResolversReadsScutilOutput(t *testing.T) {
	out := `DNS configuration

resolver #1
  nameserver[0] : 198.18.0.2
  nameserver[1] : 198.18.0.3
  if_index : 21 (utun4)

resolver #2
  domain   : local
  nameserver[0] : 127.0.0.1
`
	got := scanResolvers(out)
	want := []string{"198.18.0.2", "198.18.0.3", "127.0.0.1"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	// The loopback entry from a later resolver must not win over a reachable one.
	if a := firstUsable(got); a != "198.18.0.2" {
		t.Errorf("firstUsable = %q", a)
	}
}

func TestParseResolvConf(t *testing.T) {
	in := strings.NewReader("# comment\nsearch example.com\nnameserver 127.0.0.1\nnameserver 10.0.0.53\noptions edns0\nnameserver\n")
	got := parseResolvConf(in)
	if len(got) != 2 || got[0] != "127.0.0.1" || got[1] != "10.0.0.53" {
		t.Fatalf("got %v", got)
	}
	if a := firstUsable(got); a != "10.0.0.53" {
		t.Errorf("firstUsable = %q, want the non-loopback entry", a)
	}
}

// HostResolver must never panic and must be safe to call repeatedly; whether this machine has a
// usable resolver is not something a test can assert.
func TestHostResolverIsStableAndNeverLoopback(t *testing.T) {
	a, b := HostResolver(), HostResolver()
	if a != b {
		t.Errorf("HostResolver is not stable: %q then %q", a, b)
	}
	if a != "" {
		if ip := net.ParseIP(a); ip == nil || ip.IsLoopback() {
			t.Errorf("HostResolver returned %q, which a guest cannot use", a)
		}
	}
}
