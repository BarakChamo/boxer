package box

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/BarakChamo/boxer/internal/vm"
)

// Stable URLs for forwarded ports, through portless (github.com/vercel-labs/portless).
//
// `auto:3000` already stops two worktrees fighting over a host port, and in doing so gives every
// worktree a different one — so an agent has to be told, per worktree, where its own dev server
// is, and a person has to look it up. portless is a local reverse proxy that routes by hostname,
// and its model is already boxer's: one namespace per git worktree, with the branch as a
// subdomain. So a sandbox's port 3000 becomes https://fix-ui.myapp.localhost in the worktree on
// fix-ui, whichever host port it actually landed on.
//
// boxer does not let portless launch anything. The sandbox already publishes a host port;
// `portless alias <name> <port>` points a name at it, which is the case portless documents for
// containers. Everything here is best effort in the direction that matters: a URL that could not
// be registered is a warning, and the sandbox, its ports and every command work exactly as they
// would with URLs off.
//
// Two things portless does not do that boxer must, both found by running it rather than reading it:
//
//   - `portless get` applies the worktree prefix and `portless alias` does not. Registering the
//     name `get` was given produced a URL that `get` then pointed somewhere else — a 404. boxer
//     asks `get` for the name and passes the whole prefixed name to `alias`.
//   - The prefix only exists for a linked worktree on a named branch. A detached worktree — what
//     most orchestrators create — gets none and collides with the main checkout, and two branches
//     whose last path segment match (feat/login, fix/login) collide with each other. boxer falls
//     back to the sandbox's own two-word name for both.

// portlessBin is the portless executable; BOXER_PORTLESS overrides it, as BOXER_SMOLVM does smolvm.
func portlessBin() string {
	if p := os.Getenv("BOXER_PORTLESS"); p != "" {
		return p
	}
	return "portless"
}

// urlRoute is one registered name, as boxer remembers it. The registry is boxer's own, not
// portless's: portless has no machine-readable listing (`list --json` prints the table), and
// boxer must be able to remove exactly what it added and nothing a person added themselves.
type urlRoute struct {
	Scope string `json:"scope"`
	Guest string `json:"guest"`
	Host  string `json:"host"`
	Name  string `json:"name"` // what was passed to `portless alias`
	URL   string `json:"url"`

	made time.Time // when the route was registered
}

func urlsDir() string { return filepath.Join(stateRoot(), "urls") }

func readRoutes() []urlRoute {
	entries, err := os.ReadDir(urlsDir())
	if err != nil {
		return nil
	}
	var out []urlRoute
	for _, ent := range entries {
		if strings.Contains(ent.Name(), ".tmp-") {
			continue // being written, or left by a killed write; never a route
		}
		b, err := os.ReadFile(filepath.Join(urlsDir(), ent.Name()))
		if err != nil {
			continue
		}
		var r urlRoute
		if json.Unmarshal(b, &r) == nil && r.Name != "" {
			r.made = time.Now() // unknown age counts as new, which only delays pruning
			if info, err := ent.Info(); err == nil {
				r.made = info.ModTime()
			}
			out = append(out, r)
		}
	}
	return out
}

// URLsOf reports the URLs registered for a scope, guest port to URL. It reads boxer's registry
// and runs nothing, so `status` can print it on every call for free.
func URLsOf(key string) map[string]string {
	var out map[string]string
	for _, r := range readRoutes() {
		if r.Scope == key {
			if out == nil {
				out = map[string]string{}
			}
			out[r.Guest] = r.URL
		}
	}
	return out
}

// URLs reports where each of this sandbox's forwarded ports is reachable by name.
//
// boxer's registry is the first answer, but it lives under $XDG_STATE_HOME, and not every process
// that asks sees the same one: a harness that strips XDG_STATE_HOME from the MCP server it
// launches made `boxer_status` report the port and not the URL, and the agent — correctly — used
// what it was told. So a port the registry does not name is looked up in portless's own route
// table, which lives under $HOME, by host port. A host port belongs to exactly one sandbox, so the
// match is unambiguous.
func (e *Env) URLs(m vm.Machine) map[string]string {
	return urlsFor(m.Name, e.hostPorts(m))
}

// MachineURLs is URLs for a machine with no Env: a listing, which knows only the allocated ports.
func MachineURLs(m vm.Machine) map[string]string { return urlsFor(m.Name, PortsOf(m)) }

// ReachableURLs is every forwarded port of m as a URL a browser can open: its named URL where
// [urls] gave it one, otherwise the loopback address of its host port.
func ReachableURLs(m vm.Machine) map[string]string {
	urls := MachineURLs(m)
	for guest, host := range PortsOf(m) {
		if _, named := urls[guest]; !named {
			if urls == nil {
				urls = map[string]string{}
			}
			urls[guest] = "http://127.0.0.1:" + host
		}
	}
	return urls
}

func urlsFor(key string, ports map[string]string) map[string]string {
	out := URLsOf(key)
	var byPort map[string]string
	for guest, host := range ports {
		if _, ok := out[guest]; ok {
			continue
		}
		if byPort == nil {
			byPort = portlessRoutesByPort()
		}
		if u, ok := byPort[host]; ok {
			if out == nil {
				out = map[string]string{}
			}
			out[guest] = u
		}
	}
	return out
}

// portlessRoutesByPort reads portless's route table as host port to URL. It is portless's own
// state rather than an interface, so anything unexpected in it reads as "no route".
func portlessRoutesByPort() map[string]string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".portless")
	b, err := os.ReadFile(filepath.Join(dir, "routes.json"))
	if err != nil {
		return nil
	}
	var routes []struct {
		Hostname string `json:"hostname"`
		Port     int    `json:"port"`
	}
	if json.Unmarshal(b, &routes) != nil {
		return nil
	}
	scheme := "https"
	if t, err := os.ReadFile(filepath.Join(dir, "proxy.tls")); err == nil && strings.TrimSpace(string(t)) == "0" {
		scheme = "http"
	}
	port := ""
	if p, err := os.ReadFile(filepath.Join(dir, "proxy.port")); err == nil {
		port = strings.TrimSpace(string(p))
	}
	out := map[string]string{}
	for _, r := range routes {
		if r.Hostname == "" || r.Port == 0 {
			continue
		}
		host := r.Hostname
		if port != "" && port != "443" && port != "80" {
			host = net.JoinHostPort(r.Hostname, port)
		}
		out[strconv.Itoa(r.Port)] = scheme + "://" + host
	}
	return out
}

// ServerEnv is where this sandbox's servers are reachable from the host, as environment for a
// process in the guest: BOXER_PORTS ("3000=61234 8080=61240", guest port to host port) and, when
// names exist, BOXER_URLS ("3000=https://fix-ui.myapp.localhost:1355"). A harness running inside
// the sandbox has no `boxer` to ask and cannot reach the names itself — they are served by a proxy
// on the host's loopback — but it is the one telling a person where to look.
func (e *Env) ServerEnv(m vm.Machine) []string {
	pairs := func(m map[string]string) string {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b []string
		for _, k := range keys {
			b = append(b, k+"="+m[k])
		}
		return strings.Join(b, " ")
	}
	var env []string
	if p := e.hostPorts(m); len(p) > 0 {
		env = append(env, "BOXER_PORTS="+pairs(p))
	}
	if u := e.URLs(m); len(u) > 0 {
		env = append(env, "BOXER_URLS="+pairs(u))
	}
	return env
}

// hostPorts is every guest port this sandbox forwards and where it landed: the allocated ones
// from the machine's labels, the fixed ones from the configuration.
func (e *Env) hostPorts(m vm.Machine) map[string]string {
	out := map[string]string{}
	for _, p := range e.Cfg.Network.Ports {
		if host, guest, ok := strings.Cut(p, ":"); ok && host != "auto" {
			out[guest] = host
		}
	}
	for guest, host := range PortsOf(m) {
		out[guest] = host
	}
	return out
}

var dnsUnsafe = regexp.MustCompile(`[^a-z0-9-]+`)

// dnsLabel makes s usable as one hostname label.
func dnsLabel(s string) string {
	s = strings.Trim(dnsUnsafe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(s) > 63 {
		s = strings.Trim(s[:63], "-")
	}
	return s
}

// urlBase is the hostname every URL of this repository ends in, before the TLD: the configured
// name, or the repository's directory — the main checkout's, not this worktree's, so every
// worktree of one repository shares it and differs only in the prefix.
func (e *Env) urlBase() string {
	if e.Cfg.URLs.Name != "" {
		return dnsLabel(e.Cfg.URLs.Name)
	}
	name := ""
	if c := e.Git.CommonDir; c != "" {
		name = filepath.Base(c)
		if name == ".git" {
			name = filepath.Base(filepath.Dir(c))
		}
		name = strings.TrimSuffix(name, ".git")
	}
	if name == "" {
		name = filepath.Base(e.Scope.Root)
	}
	if b := dnsLabel(name); b != "" {
		return b
	}
	return "app"
}

// service names one guest port: the repository alone when it forwards one unlabelled port, and
// "<label>.<repo>" otherwise, which is portless's own convention for several apps in one project.
func (e *Env) service(guest string, ports int) string {
	label := dnsLabel(e.Cfg.URLs.Names[guest])
	if label == "" && ports > 1 {
		label = guest
	}
	if label == "" {
		return e.urlBase()
	}
	return label + "." + e.urlBase()
}

// portless runs one portless command in the worktree with a bound, and never with a terminal:
// a proxy start that decided to ask sudo for a password would otherwise hang a hook forever.
func portless(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, portlessBin(), args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // no controlling terminal, so no prompt
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// proxyDown reports whether portless's proxy is not running, from its own pid file. Only a
// process that is alive counts: a pid file survives the reboot that killed its process.
func proxyDown() bool {
	home, _ := os.UserHomeDir()
	b, err := os.ReadFile(filepath.Join(home, ".portless", "proxy.pid"))
	if err != nil {
		return true
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	return err != nil || pid <= 0 || syscall.Kill(pid, 0) != nil
}

// ensureProxy starts the portless proxy when it is not already running. An alias with no proxy
// behind it is a name that resolves and refuses the connection.
func ensureProxy(dir string) error {
	if !proxyDown() {
		return nil
	}
	port := os.Getenv("PORTLESS_PORT")
	if port == "" {
		port = "1355" // portless's own unprivileged default; 443 needs sudo
	}
	if n, err := strconv.Atoi(port); err == nil && n < 1024 {
		return fmt.Errorf("the portless proxy is not running, and port %d needs sudo; start it yourself: portless proxy start", n)
	}
	if out, err := portless(dir, "proxy", "start", "--port", port); err != nil {
		return fmt.Errorf("portless proxy start: %v: %s", err, lastLine(out))
	}
	_ = os.WriteFile(proxyMarker(), nil, 0o644)
	// Names asked for before the proxy listens come back with the wrong scheme.
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end) && !proxyDown(); time.Sleep(100 * time.Millisecond) {
		if c, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 200*time.Millisecond); err == nil {
			_ = c.Close()
			break
		}
	}
	return nil
}

// portlessDir is portless's own state directory. boxer keeps its two proxy files there rather than
// in its state directory, because the proxy is per user and a harness may run boxer with a
// different XDG_STATE_HOME.
func portlessDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".portless")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// proxyMarker records that boxer, not the user, started the proxy, so boxer may stop it.
func proxyMarker() string { return filepath.Join(portlessDir(), "boxer-started") }

// portlessLock serialises starting, registering on and stopping the proxy.
func portlessLock() string { return filepath.Join(portlessDir(), "boxer.lock") }

// stopIdleProxy stops a proxy boxer started once no route of anyone's is left. A proxy the user
// started, or one still serving any route, is left alone.
func stopIdleProxy() {
	if _, err := os.Stat(proxyMarker()); err != nil {
		return
	}
	unlock, err := lockFile(portlessLock())
	if err != nil {
		return
	}
	defer unlock()
	if len(portlessRoutesByPort()) > 0 {
		return
	}
	if !proxyDown() {
		if _, err := portless(os.TempDir(), "proxy", "stop"); err != nil {
			return
		}
	}
	_ = os.Remove(proxyMarker())
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// publishURLs registers a name for every forwarded port of a running sandbox and returns them,
// guest port to URL. Failures are printed and skipped; see the file comment.
func (e *Env) publishURLs(m vm.Machine, w io.Writer) map[string]string {
	if !e.Cfg.URLs.Enabled {
		return nil
	}
	ports := e.hostPorts(m)
	if len(ports) == 0 {
		return nil
	}
	if _, err := exec.LookPath(portlessBin()); err != nil {
		fmt.Fprintf(w, "boxer: warning: urls.enabled is set but portless is not installed; ports are still forwarded\n")
		fmt.Fprintf(w, "boxer: fix: npm install -g portless\n")
		return nil
	}
	// Worktrees start together under an orchestrator, and each would start the proxy and ask it
	// for names while another was still bringing it up: portless then answers http:// for a proxy
	// that serves https. One at a time per user, as portless itself is.
	if unlock, err := lockFile(portlessLock()); err == nil {
		defer unlock()
	}
	if err := ensureProxy(e.Scope.Root); err != nil {
		fmt.Fprintf(w, "boxer: warning: %v\n", err)
	}
	owner := map[string]string{} // alias name -> scope that registered it
	for _, r := range readRoutes() {
		owner[r.Name] = r.Scope
	}
	guests := make([]string, 0, len(ports))
	for g := range ports {
		guests = append(guests, g)
	}
	sort.Strings(guests)
	out := map[string]string{}
	for _, guest := range guests {
		host := ports[guest]
		u, err := e.registerURL(guest, host, len(ports), owner)
		if err != nil {
			fmt.Fprintf(w, "boxer: warning: no URL for guest port %s: %v\n", guest, err)
			continue
		}
		out[guest] = u
		fmt.Fprintf(w, "boxer: url: %s -> guest port %s\n", u, guest)
	}
	for _, u := range out {
		awaitRoute(u, 3*time.Second)
	}
	return out
}

// awaitRoute waits until the proxy serves a newly registered name. portless reloads its route
// table asynchronously, so for a moment after registration — always, when the proxy has just been
// started — the name answers portless's own 404. Handing an agent that URL means its first request
// fails. ponytail: an app whose / is itself a 404 costs the full wait once; a portless status
// endpoint would end that.
func awaitRoute(raw string, limit time.Duration) {
	u, err := url.Parse(raw)
	if err != nil {
		return
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"https": "443", "http": "80"}[u.Scheme]
	}
	c := &http.Client{
		Timeout: 500 * time.Millisecond,
		Transport: &http.Transport{
			// The proxy is on this host's loopback, whatever the name resolves to.
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
			},
			// Only a readiness probe of boxer's own local proxy, whose CA Go may not trust.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: u.Hostname()}, //nolint:gosec // loopback readiness probe
		},
	}
	defer c.CloseIdleConnections()
	for end := time.Now().Add(limit); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
		resp, err := c.Get(raw)
		if err != nil {
			return // nothing listening: no proxy to wait for, and the start already warned
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			return
		}
	}
}

// registerURL chooses and registers one name. The choice, in order: the name portless itself
// would give this worktree; for a linked worktree that portless gives no prefix (detached HEAD),
// or when another sandbox already holds that name, the sandbox's own two-word name in its place.
func (e *Env) registerURL(guest, host string, ports int, owner map[string]string) (string, error) {
	svc := e.service(guest, ports)
	got, err := portless(e.Scope.Root, "get", svc)
	if err != nil {
		return "", fmt.Errorf("portless get: %v: %s", err, lastLine(got))
	}
	u, err := url.Parse(lastLine(got))
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("portless get printed %q, not a URL", got)
	}
	hostname := u.Hostname()
	prefix, tld := "", ""
	if strings.HasPrefix(hostname, svc+".") {
		tld = strings.TrimPrefix(hostname, svc+".")
	} else if i := strings.Index(hostname, "."+svc+"."); i > 0 {
		prefix, tld = hostname[:i], hostname[i+len(svc)+2:]
	} else {
		return "", fmt.Errorf("portless named %s %q, which boxer cannot take apart", svc, hostname)
	}
	name := svc
	if prefix != "" {
		name = prefix + "." + svc
	}
	fallback := e.Scope.Slug() + "." + svc
	if (prefix == "" && e.Git.Linked) || (owner[name] != "" && owner[name] != e.Scope.Key) {
		name = fallback
	}
	if owner[name] != "" && owner[name] != e.Scope.Key {
		name = e.Scope.Key + "." + svc // two sandboxes can share a slug; they cannot share a key
	}
	args := []string{"alias", name, host}
	if owner[name] == e.Scope.Key {
		args = append(args, "--force") // our own name from an earlier boot: re-point it
	}
	if out, err := portless(e.Scope.Root, args...); err != nil {
		// Someone registered this name outside boxer. Take the sandbox's own name rather than
		// steal theirs.
		if name == fallback {
			return "", fmt.Errorf("portless alias %s: %v: %s", name, err, lastLine(out))
		}
		name = fallback
		if out, err := portless(e.Scope.Root, "alias", name, host); err != nil {
			return "", fmt.Errorf("portless alias %s: %v: %s", name, err, lastLine(out))
		}
	}
	if p := u.Port(); p != "" {
		u.Host = net.JoinHostPort(name+"."+tld, p)
	} else {
		u.Host = name + "." + tld
	}
	full := strings.TrimSuffix(u.String(), "/")
	r := urlRoute{Scope: e.Scope.Key, Guest: guest, Host: host, Name: name, URL: full}
	if err := os.MkdirAll(urlsDir(), 0o755); err == nil {
		if b, err := json.Marshal(r); err == nil {
			_ = writeAtomic(filepath.Join(urlsDir(), name), b, 0o644)
		}
	}
	owner[name] = e.Scope.Key
	return full, nil
}

// unpublishURLs removes every name boxer registered for a scope, from portless and from the
// registry. A route portless no longer has is simply forgotten.
func unpublishURLs(key string) {
	for _, r := range readRoutes() {
		if r.Scope != key {
			continue
		}
		if _, err := exec.LookPath(portlessBin()); err == nil {
			_, _ = portless(os.TempDir(), "alias", "--remove", r.Name)
		}
		if !strings.ContainsAny(r.Name, `/\`) && r.Name != "" && r.Name != "." && r.Name != ".." {
			_ = os.Remove(filepath.Join(urlsDir(), r.Name)) // the name comes from the file's JSON
		}
	}
	stopIdleProxy()
}

// removeRoutesTo removes every portless route that points at one of these host ports. They are a
// deleted sandbox's ports, so a route to them is that sandbox's whoever registered it.
func removeRoutesTo(ports map[string]string) {
	if _, err := exec.LookPath(portlessBin()); err != nil {
		return
	}
	mine := map[string]bool{}
	for _, h := range ports {
		mine[h] = true
	}
	for host, u := range portlessRoutesByPort() {
		if !mine[host] {
			continue
		}
		pu, err := url.Parse(u)
		if err != nil {
			continue
		}
		// portless takes the name without its TLD, which is the last label.
		// ponytail: assumes a one-label TLD (localhost, test); under a multi-label PORTLESS_TLD
		// the removal misses, and the registry path above is the only one that works.
		name := pu.Hostname()
		if i := strings.LastIndex(name, "."); i > 0 {
			name = name[:i]
		}
		_, _ = portless(os.TempDir(), "alias", "--remove", name)
	}
	stopIdleProxy()
}

// PruneURLsAfter is how old a route must be before gc prunes it. gc's list of live sandboxes is
// taken before it reads the routes, so a sandbox created in between has routes and no entry.
const PruneURLsAfter = time.Hour

// PruneURLs removes routes older than minAge whose sandbox is not in live and whose host port
// nothing is listening on. Both, because live only covers the configured backend: a route to
// another backend's running sandbox still has a listener, and is kept.
func PruneURLs(live map[string]bool, minAge time.Duration) int {
	n := 0
	for _, r := range readRoutes() {
		if live[r.Scope] || time.Since(r.made) < minAge {
			continue
		}
		if c, err := net.DialTimeout("tcp", "127.0.0.1:"+r.Host, 200*time.Millisecond); err == nil {
			_ = c.Close()
			continue
		}
		unpublishURLs(r.Scope)
		n++
	}
	return n
}
