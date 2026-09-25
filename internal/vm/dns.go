package vm

import (
	"bufio"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// smolvm points a networked guest at public resolvers (8.8.8.8 / 1.1.1.1) unless told otherwise.
// That is a reasonable default for a tool that cannot know anything about the host, and it is
// expensive here: every lookup is an internet round trip over the guest's user-mode network stack,
// nothing caches, and a repeated lookup costs the same as the first. Measured in a boxer guest on
// an M4, every name took ~415ms, including one resolved a moment earlier.
//
// The host already runs a caching resolver — that is what the rest of the machine uses — and
// pointing the guest at it takes the same lookups to 2-3ms, and to 0ms once cached. Next.js's dev
// server alone resolves several names while starting, so this is seconds on a bring-up, and it is
// the difference between `npm install` feeling like a container and feeling like a VM.
//
// The one way this makes things worse is a resolver the guest cannot reach, which would turn slow
// DNS into no DNS. Two guards: a loopback address is never used, because in the guest it means the
// guest itself; and `network.dns = "off"` hands the decision back to smolvm. Anything else is
// passed through as given.

var hostDNSOnce struct {
	sync.Once
	addr string
}

// HostResolver returns the host's first usable DNS server, or "" when there is none worth passing
// to a guest. Looked up once per process: it involves a subprocess on macOS and cannot change
// usefully mid-command.
func HostResolver() string {
	hostDNSOnce.Do(func() { hostDNSOnce.addr = findHostResolver() })
	return hostDNSOnce.addr
}

func findHostResolver() string {
	// macOS keeps the real resolver list in the system configuration, not in /etc/resolv.conf,
	// which is often a stub. `scutil --dns` prints the resolvers in priority order.
	if out, err := exec.Command("scutil", "--dns").Output(); err == nil {
		if a := firstUsable(scanResolvers(string(out))); a != "" {
			return a
		}
	}
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	return firstUsable(parseResolvConf(f))
}

// parseResolvConf returns the nameservers in a resolv.conf, in file order.
func parseResolvConf(r io.Reader) []string {
	var found []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == "nameserver" {
			found = append(found, fields[1])
		}
	}
	return found
}

// scanResolvers pulls every `nameserver[n] : addr` out of scutil's output.
func scanResolvers(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "nameserver[") {
			continue
		}
		if i := strings.Index(line, ":"); i >= 0 {
			out = append(out, strings.TrimSpace(line[i+1:]))
		}
	}
	return out
}

// firstUsable returns the first address a guest could actually reach. Loopback is the important
// exclusion: a host running dnsmasq or systemd-resolved advertises 127.0.0.1, which inside the
// guest names the guest. Link-local and unspecified addresses are no better.
func firstUsable(addrs []string) string {
	for _, a := range addrs {
		ip := net.ParseIP(strings.TrimSpace(a))
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
			continue
		}
		return ip.String()
	}
	return ""
}
