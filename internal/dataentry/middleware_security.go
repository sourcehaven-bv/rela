package dataentry

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// SecurityConfig configures the HTTP security middlewares.
//
// rela-server is intended to run on a local port, but a browser visiting any
// other site is already inside the loopback trust boundary. These middlewares
// reject:
//
//   - Requests whose Host header is not in the loopback allowlist
//     (DNS rebinding defense).
//   - Requests to sensitive endpoints whose Origin (or Referer fallback) is
//     not in the allowlist (CSRF / cross-origin read defense).
//
// All sensitive endpoints are protected on every method, not just non-safe
// ones, because some handlers (e.g. /api/command/) historically accept GET
// for state-changing operations and a method-based filter would miss
// `<img src=...>` style attacks.
type SecurityConfig struct {
	// BindAddress is the host:port (or :port) the server is bound to.
	// Used to derive the default Host and Origin allowlists.
	BindAddress string
	// AllowedOrigins are extra origins permitted in addition to the
	// loopback defaults derived from BindAddress. Used to allow dev servers
	// such as Vite running on a different port.
	AllowedOrigins []string
}

// security is the runtime state computed from a SecurityConfig.
type security struct {
	// allowedHosts is the set of acceptable Host header values, lowercased.
	// Empty (nil or zero-length) means "accept any Host" — only used when
	// the operator binds to an unspecified address (0.0.0.0 / ::), where
	// the legitimate Host depends on which interface and DNS name the
	// client used and cannot be enumerated ahead of time.
	allowedHosts map[string]struct{}
	// allowedOrigins is the set of acceptable Origin/Referer values for
	// sensitive paths, normalized via normaliseOrigin.
	allowedOrigins map[string]struct{}
}

// newSecurity prepares the host and origin allowlists from cfg.
func newSecurity(cfg SecurityConfig) (*security, error) {
	host, port, err := splitBindAddress(cfg.BindAddress)
	if err != nil {
		return nil, err
	}

	allowedHosts := make(map[string]struct{})
	addHost := func(h string) {
		allowedHosts[strings.ToLower(h)] = struct{}{}
	}

	allowedOrigins := make(map[string]struct{})
	addOrigin := func(o string) {
		allowedOrigins[o] = struct{}{}
	}

	switch {
	case isUnspecified(host):
		// Operator opted into LAN access. We cannot enumerate the
		// legitimate Host headers ahead of time (each interface, each
		// DNS name, each /etc/hosts entry would need to be allowed),
		// so we leave allowedHosts nil to accept any Host. The Origin
		// allowlist is the operator's responsibility via --allowed-origin.
		allowedHosts = nil

	case isLoopback(host):
		addHost(net.JoinHostPort("127.0.0.1", port))
		addHost(net.JoinHostPort("localhost", port))
		addHost(net.JoinHostPort("::1", port))
		addOrigin("http://" + net.JoinHostPort("127.0.0.1", port))
		addOrigin("http://" + net.JoinHostPort("localhost", port))
		addOrigin("http://" + net.JoinHostPort("::1", port))

	default:
		// Bound to a specific non-loopback interface. Allow that exact
		// host:port for both Host and Origin.
		addHost(net.JoinHostPort(host, port))
		addOrigin("http://" + net.JoinHostPort(host, port))
	}

	for _, raw := range cfg.AllowedOrigins {
		o, parseErr := normaliseOrigin(raw)
		if parseErr != nil {
			return nil, parseErr
		}
		addOrigin(o)
	}

	return &security{
		allowedHosts:   allowedHosts,
		allowedOrigins: allowedOrigins,
	}, nil
}

// requireLocalHost rejects requests whose Host header is not in the allowlist.
// This defends against DNS rebinding: even after the attacker rebinds their
// hostname to 127.0.0.1, the browser still sends Host: attacker.example.
//
// When allowedHosts is nil (operator bound to an unspecified address) the
// check is bypassed entirely. The Origin allowlist remains in force and is
// the only CSRF gate in that mode.
func (s *security) requireLocalHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.allowedHosts != nil {
			if _, ok := s.allowedHosts[strings.ToLower(r.Host)]; !ok {
				s.reject(w, r, "host_not_allowed")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// AllowFullScriptDetail reports whether the caller of r should receive the
// rich script-error envelope (source slice, full stack, captured print()
// output). Returns true only for loopback peers. Behind a reverse proxy
// this fails closed (the proxy IP is non-loopback) — the right default
// since the data-entry server has no auth layer; rich envelopes would
// otherwise leak script source and captured print() output across the
// LAN. Intentionally does not honor X-Forwarded-For; any future
// proxy-aware middleware must keep this gate honest.
func (s *security) AllowFullScriptDetail(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return isLoopback(host)
}

// requireSameOrigin rejects sensitive requests whose Origin (or Referer fallback)
// is not in the allowlist. Applies to every HTTP method on the configured
// sensitive paths, because some endpoints (notably /api/command/) accept GET
// and a method-based filter would miss `<img src=...>` style attacks.
func (s *security) requireSameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isSensitivePath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		origin, ok := requestOrigin(r)
		if !ok {
			s.reject(w, r, "origin_missing")
			return
		}
		if _, allowed := s.allowedOrigins[origin]; !allowed {
			s.reject(w, r, "origin_not_allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// reject writes a 403 response in a consistent format and logs the rejection.
//
// All header values are quoted via strconv.Quote so an attacker-controlled
// Origin/Referer/Host with embedded newlines cannot inject fake log lines
// into structured log destinations.
func (s *security) reject(w http.ResponseWriter, r *http.Request, reason string) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	slog.Warn("security blocked request",
		"rule", reason,
		"host", strconv.Quote(truncate(r.Host, 200)),
		"origin", strconv.Quote(truncate(origin, 200)),
		"path", strconv.Quote(truncate(r.URL.Path, 200)),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":  "forbidden",
		"reason": reason,
	})
}

// sensitivePathPrefixes lists the path prefixes protected by requireSameOrigin.
//
// Read-only public endpoints (static assets, the SPA shell) are intentionally
// excluded — they have the lighter Host-only protection so a malicious page
// cannot infer that the server exists, but their content carries no project
// data and is safe to fetch cross-origin. Everything that reads or writes
// project state lives behind one of these prefixes.
var sensitivePathPrefixes = []string{
	"/api/", // covers /api/v1/* and all sub-paths including SSE
}

// insensitivePathPrefixes carves exceptions OUT of the sensitive prefixes above.
//
// Custom-app files (`/api/v1/_apps/<id>/...`) must NOT be same-origin gated:
// they load into a sandboxed iframe via `src=` and as sub-resources (the app's
// own js/css/images, and the reserved _rela.js SDK), all of which the browser
// fetches with NO Origin (top-level iframe navigation) or a stripped/`null`
// one — requireSameOrigin would 403 them as `origin_missing` and the app would
// never load. This is safe: app files carry no project data (they ARE the app's
// own static assets — an attacker fetching them learns only the app's source,
// which is project config, not user data), and each response carries the
// locking path-scoped CSP. The actual data surface (`/api/v1/*` proper) stays
// gated, and the app's iframe cannot reach it (CSP connect-src 'none' → only
// the same-origin host page's MessageChannel bridge talks to the API).
var insensitivePathPrefixes = []string{
	"/api/v1/_apps/",
}

func isSensitivePath(path string) bool {
	for _, p := range insensitivePathPrefixes {
		if strings.HasPrefix(path, p) {
			return false
		}
	}
	for _, p := range sensitivePathPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// requestOrigin returns the request's effective origin, normalised to
// lowercase scheme and host with default ports made explicit. The second
// return value is false if no origin could be determined or the value is
// the literal "null" sent by sandboxed iframes.
func requestOrigin(r *http.Request) (string, bool) {
	if origin := r.Header.Get("Origin"); origin != "" {
		if origin == "null" {
			return "", false
		}
		o, err := normaliseOrigin(origin)
		if err != nil {
			return "", false
		}
		return o, true
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		// Referer is a full URL with a path; extract just the origin.
		o, err := originFromURL(referer)
		if err != nil {
			return "", false
		}
		return o, true
	}
	return "", false
}

// originFromURL parses a full URL (e.g. a Referer header) and returns its
// scheme://host:port origin, normalised the same way as normaliseOrigin.
func originFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", errMalformedOrigin
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", errMalformedOrigin
		}
	}
	return scheme + "://" + net.JoinHostPort(host, port), nil
}

// normaliseOrigin parses a URL and returns scheme://host:port with the
// scheme and hostname lowercased and the default port (80 for http,
// 443 for https) made explicit. Anything with a path, query, or fragment
// (other than a single trailing "/") is rejected as malformed per RFC 6454.
func normaliseOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", errMalformedOrigin
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", errMalformedOrigin
		}
	}
	// RFC 6454: an origin has scheme, host, port — no path, query, fragment.
	// We tolerate a single empty path (Origin headers from some clients).
	if u.Path != "" && u.Path != "/" {
		return "", errMalformedOrigin
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", errMalformedOrigin
	}
	return scheme + "://" + net.JoinHostPort(host, port), nil
}

// errMalformedOrigin is returned by normaliseOrigin for unparseable input.
// Defined as a sentinel so callers can distinguish from network parse errors.
var errMalformedOrigin = &originError{"malformed origin"}

type originError struct{ msg string }

func (e *originError) Error() string { return e.msg }

// splitBindAddress accepts ":8080", "127.0.0.1:8080", or "[::1]:8080".
//
// An empty host (":port") means "listen on all interfaces" — same as
// "0.0.0.0:port" — and we preserve that intent so the caller can detect
// the unspecified address case via isUnspecified.
func splitBindAddress(addr string) (host, port string, err error) {
	if strings.HasPrefix(addr, ":") {
		return "0.0.0.0", strings.TrimPrefix(addr, ":"), nil
	}
	host, port, err = net.SplitHostPort(addr)
	if err != nil {
		return "", "", err
	}
	if host == "" {
		host = "0.0.0.0"
	}
	return host, port, nil
}

// isLoopback reports whether the given host is a loopback name or IP.
func isLoopback(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// isUnspecified reports whether host is the IPv4 or IPv6 unspecified address
// (the "any" interface). When the server is bound to one of these, the Host
// allowlist must be relaxed because the legitimate Host depends entirely on
// how clients address the machine.
func isUnspecified(host string) bool {
	if host == "" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsUnspecified()
	}
	return false
}

// truncate returns s truncated to at most n runes, with "…" appended if cut.
// Rune-aware so multibyte UTF-8 strings are not sliced mid-codepoint.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// isSafePathSegment reports whether s is safe to embed in a filesystem path
// component. It allows ASCII alphanumerics, hyphen, underscore, and dot but
// rejects path separators, leading dot, `..`, NUL, control characters, and
// the empty string. Use it to validate path segments taken from URLs before
// they are joined into filenames.
func isSafePathSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	if strings.HasPrefix(s, ".") {
		// Reject hidden-style segments to avoid surprises like ".git".
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}
