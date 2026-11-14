package lua

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// HTTPPolicy decides which addresses the Lua http.* bindings may connect to.
// It is a value set once at wiring: [SetHTTPPolicy] for the process-wide
// client, or [NewHTTPClient] for a client handed to [WithHTTPClient].
//
// The check runs at dial time, on the IP address the connection is actually
// opened to. A hostname is therefore judged by what it resolves to at the
// moment of the dial, which covers DNS rebinding, and every address a
// multi-address name resolves to is judged on its own.
//
// Two levels apply:
//
//   - Always, in every mode: link-local (169.254.0.0/16, fe80::/10, which
//     holds the cloud metadata address 169.254.169.254), the AWS IPv6
//     metadata address fd00:ec2::254, unspecified (0.0.0.0, ::), multicast
//     and broadcast are refused, also in IPv4-mapped, NAT64 and 6to4 form.
//     No documented rela use reaches these, and the metadata service hands
//     out cloud credentials.
//   - Strict, set for tenant mode: every address that is not global unicast
//     is refused as well. That adds loopback, private (10/8, 172.16/12,
//     192.168/16, fc00::/7), CGNAT (100.64/10) and the IANA special-purpose
//     ranges. A tenant's script must not reach the host or the operator's
//     internal network. [HTTPPolicy.Allow] and [HTTPPolicy.AllowHosts]
//     reopen named destinations, such as an internal identity provider.
//
// The single-tenant default is not strict, because the documented
// behavior of http.* includes reaching localhost and private hosts.
//
// Proxies: in strict mode the HTTP(S)_PROXY environment variables are
// ignored, because the proxy would reach the destination on the script's
// behalf and the dial-time check would only see the proxy. Outside strict
// mode the environment proxy is honored, and the always-on check then
// applies to the connection to the proxy only.
//
// The zero value is the single-tenant default.
type HTTPPolicy struct {
	// Strict refuses every non-global address outside the allowlists.
	Strict bool

	// Allow lists address ranges that stay reachable in strict mode. It
	// does not reopen an always-refused address.
	//
	// Nil: accepted — nothing beyond global unicast is reachable.
	Allow []netip.Prefix

	// AllowHosts lists "host:port" destinations, as named in the URL, that
	// stay reachable in strict mode whatever they resolve to. The port is
	// required: http and https default to 80 and 443. The always-on check
	// still applies to the resolved address.
	//
	// Nil: accepted — no host is exempt.
	AllowHosts []string
}

// errDestinationNotAllowed is the error a refused dial returns. Its text
// names no address: the resolved IP of a name the script gave may itself be
// internal information.
var errDestinationNotAllowed = errors.New("http: destination address not allowed")

// ParseHTTPAllowList splits operator allowlist entries into the prefixes and
// host:port destinations of an [HTTPPolicy]. An entry is a CIDR
// ("10.1.0.0/16"), a bare IP ("10.1.2.3", read as a single-address prefix)
// or a host:port ("idp.internal:8443").
func ParseHTTPAllowList(entries []string) (prefixes []netip.Prefix, hostPorts []string, err error) {
	for _, raw := range entries {
		e := strings.TrimSpace(raw)
		if e == "" {
			continue
		}
		if p, perr := netip.ParsePrefix(e); perr == nil {
			prefixes = append(prefixes, p.Masked())
			continue
		}
		if a, aerr := netip.ParseAddr(e); aerr == nil {
			prefixes = append(prefixes, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
			continue
		}
		hp, herr := normalizeHostPort(e)
		if herr != nil {
			return nil, nil, fmt.Errorf("http allowlist entry %q: want a CIDR, an IP or host:port", e)
		}
		hostPorts = append(hostPorts, hp)
	}
	return prefixes, hostPorts, nil
}

// normalizeHostPort lower-cases the host of a host:port and checks the port.
func normalizeHostPort(s string) (string, error) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return "", err
	}
	if host == "" {
		return "", errors.New("empty host")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", errors.New("bad port")
	}
	return net.JoinHostPort(strings.ToLower(host), port), nil
}

// sharedHTTPClient is the process-wide client of the http.* bindings, used
// by every runtime that was not given one with [WithHTTPClient]. Process
// global rather than per-Runtime so connection pooling holds across
// short-lived runtimes (CLI invocations, scheduler ticks, MCP tool calls),
// and so a surface that builds its own deps still gets the policy.
var sharedHTTPClient atomic.Pointer[http.Client]

// httpPolicySet records that [SetHTTPPolicy] ran.
var httpPolicySet atomic.Bool

func init() {
	c, err := NewHTTPClient(HTTPPolicy{})
	if err != nil {
		panic(err) // coverage-ignore: the zero policy always builds
	}
	sharedHTTPClient.Store(c)
}

// SetHTTPPolicy replaces the process-wide http.* client with one that
// enforces p. Call it once at startup, before any script runs; a second
// call is an error, so two wiring sites cannot disagree silently.
func SetHTTPPolicy(p HTTPPolicy) error {
	c, err := NewHTTPClient(p)
	if err != nil {
		return err
	}
	if !httpPolicySet.CompareAndSwap(false, true) {
		return errors.New("lua: the HTTP policy is already set")
	}
	sharedHTTPClient.Store(c)
	return nil
}

// NewHTTPClient returns a client for the http.* bindings that enforces p.
// Redirects are not followed, so scripts handle them explicitly.
//
// The Transport is explicitly bounded rather than reusing
// http.DefaultTransport: rela's scheduler and MCP server are long-running
// processes, and a script that fans out to many distinct hosts would
// otherwise grow idle connections without bound. MaxResponseHeaderBytes
// also caps header memory before the 10 MiB body cap fires.
func NewHTTPClient(p HTTPPolicy) (*http.Client, error) {
	d, err := newPolicyDialer(p)
	if err != nil {
		return nil, err
	}
	proxy := http.ProxyFromEnvironment
	if p.Strict {
		proxy = nil
	}
	return &http.Client{
		Timeout: httpDefaultTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			Proxy:                  proxy,
			DialContext:            d.DialContext,
			MaxIdleConns:           100,
			MaxIdleConnsPerHost:    10,
			MaxConnsPerHost:        50,
			IdleConnTimeout:        90 * time.Second,
			TLSHandshakeTimeout:    10 * time.Second,
			ExpectContinueTimeout:  1 * time.Second,
			ResponseHeaderTimeout:  30 * time.Second,
			MaxResponseHeaderBytes: 1 << 20, // 1 MiB
			ForceAttemptHTTP2:      true,
		},
	}, nil
}

// policyDialer dials through one of two dialers whose Control hooks check
// the resolved address: checked applies the full policy, exempt (for an
// AllowHosts destination) only the always-on part.
type policyDialer struct {
	checked    *net.Dialer
	exempt     *net.Dialer
	allowHosts map[string]bool
}

func newPolicyDialer(p HTTPPolicy) (*policyDialer, error) {
	hosts := make(map[string]bool, len(p.AllowHosts))
	for _, h := range p.AllowHosts {
		hp, err := normalizeHostPort(h)
		if err != nil {
			return nil, fmt.Errorf("http allowlist host %q: %w", h, err)
		}
		hosts[hp] = true
	}
	full := addrPolicy{strict: p.Strict, allow: append([]netip.Prefix(nil), p.Allow...)}
	return &policyDialer{
		checked:    newControlledDialer(full),
		exempt:     newControlledDialer(addrPolicy{}),
		allowHosts: hosts,
	}, nil
}

func newControlledDialer(ap addrPolicy) *net.Dialer {
	return &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   ap.control,
	}
}

// DialContext dials addr, the URL's host:port (or the proxy's), through the
// dialer its allowlist entry selects.
func (d *policyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if len(d.allowHosts) > 0 {
		if hp, err := normalizeHostPort(addr); err == nil && d.allowHosts[hp] {
			return d.exempt.DialContext(ctx, network, addr)
		}
	}
	return d.checked.DialContext(ctx, network, addr)
}

// addrPolicy judges one resolved address.
type addrPolicy struct {
	strict bool
	allow  []netip.Prefix
}

// control is the net.Dialer Control hook. address is the resolved ip:port
// the socket is about to connect to. An address that does not parse is
// refused: failing open here would undo the whole check.
func (p addrPolicy) control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil || !p.permits(ap.Addr()) {
		return errDestinationNotAllowed
	}
	return nil
}

// permits reports whether a connection to a may be opened.
func (p addrPolicy) permits(a netip.Addr) bool {
	a = a.WithZone("").Unmap()
	if alwaysRefused(a) {
		return false
	}
	if !p.strict || !strictRefused(a) {
		return true
	}
	for _, pre := range p.allow {
		if pre.Contains(a) {
			return true
		}
	}
	return false
}

var (
	// awsIPv6Metadata is the IPv6 address of the EC2 instance metadata
	// service. It lies in fc00::/7, which only strict mode refuses, so it
	// is named on its own.
	awsIPv6Metadata = netip.MustParseAddr("fd00:ec2::254")
	ipv4Broadcast   = netip.MustParseAddr("255.255.255.255")
	nat64Prefix     = netip.MustParsePrefix("64:ff9b::/96")
	sixToFourPrefix = netip.MustParsePrefix("2002::/16")
)

// alwaysRefused reports the addresses no mode may reach. a is unmapped.
func alwaysRefused(a netip.Addr) bool {
	if !a.IsValid() || a.IsUnspecified() || a.IsLinkLocalUnicast() || a.IsMulticast() ||
		a == ipv4Broadcast || a == awsIPv6Metadata {

		return true
	}
	if v4, ok := embeddedIPv4(a); ok {
		return alwaysRefused(v4)
	}
	return false
}

// strictSpecialPrefixes are non-global ranges that netip's predicates do not
// cover: CGNAT and the IANA special-purpose registries.
var strictSpecialPrefixes = mustPrefixes(
	"0.0.0.0/8",       // "this network"
	"100.64.0.0/10",   // CGNAT, RFC 6598
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // TEST-NET-1
	"192.88.99.0/24",  // 6to4 relay anycast
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"240.0.0.0/4",     // reserved
	"::/96",           // IPv4-compatible (deprecated)
	"64:ff9b:1::/48",  // local-use NAT64
	"100::/64",        // discard-only
	"2001::/23",       // IETF protocol assignments (Teredo among them)
	"2001:db8::/32",   // documentation
	"fec0::/10",       // site-local (deprecated)
)

// strictRefused reports the addresses strict mode refuses on top of
// [alwaysRefused]. a is unmapped.
func strictRefused(a netip.Addr) bool {
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() {
		return true
	}
	if v4, ok := embeddedIPv4(a); ok {
		return strictRefused(v4)
	}
	for _, p := range strictSpecialPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// embeddedIPv4 returns the IPv4 address a NAT64 (64:ff9b::/96) or 6to4
// (2002::/16) address routes to.
func embeddedIPv4(a netip.Addr) (netip.Addr, bool) {
	if !a.Is6() {
		return netip.Addr{}, false
	}
	b := a.As16()
	switch {
	case nat64Prefix.Contains(a):
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}), true
	case sixToFourPrefix.Contains(a):
		return netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}), true
	}
	return netip.Addr{}, false
}

func mustPrefixes(ss ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}
