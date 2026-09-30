package config

import (
	"fmt"
	"net/netip"
	"strings"
)

// Egress is an allowlist split into the two kinds of rule smolvm enforces: hostnames, which it
// resolves when the VM starts and also lets through its DNS filter, and address ranges, which
// it allows on every port without any DNS.
type Egress struct {
	Hosts []string
	CIDRs []string
}

// SplitAllow classifies allowlist entries. An entry is one of:
//
//	registry.npmjs.org        a hostname, matched exactly
//	10.1.2.3, fd00::1         one address
//	10.0.0.0/8, fd00::/64     a CIDR block
//	10.0.0.10-10.0.0.40       an inclusive address range, turned into the fewest CIDR blocks
//
// Wildcards and ports are refused, by name. smolvm has neither: it accepts `*.example.com` and
// then allows nothing, and refuses `host:443` outright. An allowlist that quietly allows less than
// it says fails every install for a reason nobody can see, so boxer says so at load instead.
func SplitAllow(entries []string) (Egress, error) {
	var e Egress
	seen := map[string]bool{}
	add := func(list *[]string, v string) {
		if !seen[v] {
			seen[v] = true
			*list = append(*list, v)
		}
	}
	for _, raw := range entries {
		hosts, cidrs, err := parseAllow(raw)
		if err != nil {
			return Egress{}, err
		}
		for _, h := range hosts {
			add(&e.Hosts, h)
		}
		for _, c := range cidrs {
			add(&e.CIDRs, c)
		}
	}
	return e, nil
}

func parseAllow(raw string) (hosts, cidrs []string, err error) {
	s := strings.TrimSpace(raw)
	switch {
	case s == "":
		return nil, nil, fmt.Errorf("network.allow_hosts has an empty entry")
	case strings.Contains(s, "*"):
		return nil, nil, fmt.Errorf("network.allow_hosts entry %q: wildcards are not supported, because smolvm allows exact hostnames only. List each host, use an allow_presets entry, or give an address range", raw)
	}
	if p, err := netip.ParsePrefix(s); err == nil {
		if p.Masked() != p {
			return nil, nil, fmt.Errorf("network.allow_hosts entry %q has bits set past the prefix; did you mean %s?", raw, p.Masked())
		}
		return nil, []string{p.String()}, nil
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return nil, []string{netip.PrefixFrom(a, a.BitLen()).String()}, nil
	}
	if _, err := netip.ParseAddrPort(s); err == nil || hasPort(s) {
		return nil, nil, fmt.Errorf("network.allow_hosts entry %q: ports are not supported; an allowed host or range is reachable on every port. Drop the port", raw)
	}
	if lo, hi, ok := strings.Cut(s, "-"); ok {
		a, errA := netip.ParseAddr(strings.TrimSpace(lo))
		b, errB := netip.ParseAddr(strings.TrimSpace(hi))
		if errA == nil || errB == nil {
			if errA != nil || errB != nil || a.Is4() != b.Is4() {
				return nil, nil, fmt.Errorf("network.allow_hosts entry %q: a range is two addresses of the same family, first-last", raw)
			}
			if b.Less(a) {
				return nil, nil, fmt.Errorf("network.allow_hosts entry %q: the range ends before it starts", raw)
			}
			return nil, rangeCIDRs(a, b), nil
		}
	}
	h := strings.TrimSuffix(s, ".")
	if !validHostname(h) {
		return nil, nil, fmt.Errorf("network.allow_hosts entry %q is not a hostname, an address, a CIDR block or an address range", raw)
	}
	return []string{strings.ToLower(h)}, nil, nil
}

// hasPort reports a hostname followed by :port or :port-port.
func hasPort(s string) bool {
	host, port, ok := strings.Cut(s, ":")
	if !ok || host == "" || strings.Contains(port, ":") {
		return false
	}
	port = strings.Replace(port, "-", "", 1)
	return port != "" && strings.Trim(port, "0123456789") == ""
}

func validHostname(h string) bool {
	if h == "" || len(h) > 253 {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

// rangeCIDRs covers first..last, inclusive, with the fewest aligned blocks: at each step, the
// largest block that starts at the current address and does not pass the end.
func rangeCIDRs(first, last netip.Addr) []string {
	var out []string
	bits := first.BitLen()
	for cur := first; ; {
		p := bits
		for p > 0 {
			wider := netip.PrefixFrom(cur, p-1).Masked()
			if wider.Addr() != cur || lastAddr(wider).Compare(last) > 0 {
				break
			}
			p--
		}
		block := netip.PrefixFrom(cur, p)
		out = append(out, block.String())
		end := lastAddr(block)
		if end.Compare(last) >= 0 {
			return out
		}
		cur = end.Next()
	}
}

// lastAddr is the highest address in p.
func lastAddr(p netip.Prefix) netip.Addr {
	b := p.Addr().AsSlice()
	for i := p.Bits(); i < len(b)*8; i++ {
		b[i/8] |= 1 << (7 - i%8)
	}
	a, _ := netip.AddrFromSlice(b)
	return a
}
