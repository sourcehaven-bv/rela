package lua

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestAddrPolicy_Permits(t *testing.T) {
	t.Parallel()
	allow := []netip.Prefix{netip.MustParsePrefix("10.20.0.0/16"), netip.MustParsePrefix("fd12::/16")}
	tests := []struct {
		addr          string
		defaultOK     bool
		strictOK      bool
		strictAllowOK bool
	}{
		// Public addresses: reachable in every mode.
		{"93.184.216.34", true, true, true},
		{"2606:2800:220:1::1", true, true, true},
		{"::ffff:93.184.216.34", true, true, true},
		{"64:ff9b::5db8:d822", true, true, true}, // NAT64 of 93.184.216.34

		// Always refused.
		{"169.254.169.254", false, false, false}, // cloud metadata
		{"169.254.1.1", false, false, false},
		{"::ffff:169.254.169.254", false, false, false}, // v4-mapped metadata
		{"64:ff9b::a9fe:a9fe", false, false, false},     // NAT64 metadata
		{"2002:a9fe:a9fe::1", false, false, false},      // 6to4 metadata
		{"fd00:ec2::254", false, false, false},          // AWS IPv6 metadata
		{"fe80::1", false, false, false},
		{"fe80::1%en0", false, false, false},
		{"0.0.0.0", false, false, false},
		{"::", false, false, false},
		{"::ffff:0.0.0.0", false, false, false},
		{"224.0.0.1", false, false, false},
		{"ff02::1", false, false, false},
		{"255.255.255.255", false, false, false},

		// Strict only.
		{"127.0.0.1", true, false, false},
		{"127.5.5.5", true, false, false},
		{"::1", true, false, false},
		{"::ffff:127.0.0.1", true, false, false},
		{"10.0.0.1", true, false, false},
		{"10.20.3.4", true, false, true}, // in the allowlist
		{"172.16.0.1", true, false, false},
		{"172.31.255.255", true, false, false},
		{"192.168.1.1", true, false, false},
		{"::ffff:192.168.1.1", true, false, false},
		{"100.64.0.1", true, false, false}, // CGNAT
		{"100.127.255.255", true, false, false},
		{"fc00::1", true, false, false},
		{"fd12::1", true, false, true}, // in the allowlist
		{"198.18.0.1", true, false, false},
		{"192.0.2.1", true, false, false},
		{"240.0.0.1", true, false, false},
		{"2001:db8::1", true, false, false},
		{"64:ff9b::a00:1", true, false, false}, // NAT64 of 10.0.0.1
		{"2002:7f00:1::1", true, false, false}, // 6to4 of 127.0.0.1
		{"::7f00:1", true, false, false},       // IPv4-compatible 127.0.0.1
	}
	def := addrPolicy{}
	strict := addrPolicy{strict: true}
	strictAllow := addrPolicy{strict: true, allow: allow}
	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			t.Parallel()
			a := netip.MustParseAddr(tc.addr)
			if got := def.permits(a); got != tc.defaultOK {
				t.Errorf("default: permits = %v, want %v", got, tc.defaultOK)
			}
			if got := strict.permits(a); got != tc.strictOK {
				t.Errorf("strict: permits = %v, want %v", got, tc.strictOK)
			}
			if got := strictAllow.permits(a); got != tc.strictAllowOK {
				t.Errorf("strict+allow: permits = %v, want %v", got, tc.strictAllowOK)
			}
		})
	}
}

func TestAddrPolicy_ControlRefusesUnparsable(t *testing.T) {
	t.Parallel()
	if err := (addrPolicy{}).control("tcp", "not-an-address", nil); err == nil {
		t.Fatal("control accepted an address that does not parse")
	}
}

func TestParseHTTPAllowList(t *testing.T) {
	t.Parallel()
	prefixes, hosts, err := ParseHTTPAllowList([]string{
		"10.1.0.0/16", " 10.2.3.4 ", "fd00::/8", "IdP.Internal:8443", "",
	})
	if err != nil {
		t.Fatalf("ParseHTTPAllowList: %v", err)
	}
	wantPrefixes := []string{"10.1.0.0/16", "10.2.3.4/32", "fd00::/8"}
	if len(prefixes) != len(wantPrefixes) {
		t.Fatalf("prefixes = %v, want %v", prefixes, wantPrefixes)
	}
	for i, p := range prefixes {
		if p.String() != wantPrefixes[i] {
			t.Errorf("prefix %d = %s, want %s", i, p, wantPrefixes[i])
		}
	}
	if len(hosts) != 1 || hosts[0] != "idp.internal:8443" {
		t.Errorf("hosts = %v, want [idp.internal:8443]", hosts)
	}

	for _, bad := range []string{"idp.internal", "idp:0", "idp:x", ":80", "10.0.0.0/33"} {
		if _, _, err := ParseHTTPAllowList([]string{bad}); err == nil {
			t.Errorf("ParseHTTPAllowList(%q) accepted a bad entry", bad)
		}
	}
}

func TestNewHTTPClient_RejectsBadAllowHost(t *testing.T) {
	t.Parallel()
	if _, err := NewHTTPClient(HTTPPolicy{AllowHosts: []string{"no-port"}}); err == nil {
		t.Fatal("NewHTTPClient accepted a host without a port")
	}
}

func TestNewHTTPClient_StrictIgnoresEnvProxy(t *testing.T) {
	t.Parallel()
	for _, strict := range []bool{false, true} {
		c, err := NewHTTPClient(HTTPPolicy{Strict: strict})
		if err != nil {
			t.Fatal(err)
		}
		tr, ok := c.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("transport is %T", c.Transport)
		}
		if got := tr.Proxy != nil; got == strict {
			t.Errorf("strict=%v: proxy set = %v", strict, got)
		}
	}
}

// newPolicyRuntime builds an http-capable runtime whose client enforces p.
func newPolicyRuntime(t *testing.T, p HTTPPolicy) *Runtime {
	t.Helper()
	c, err := NewHTTPClient(p)
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	var buf bytes.Buffer
	rt := NewReader(ReadDeps{World: store.TrivialScope()}, &buf,
		WithCapabilities(Capabilities{HTTP: true}), WithHTTPClient(c))
	t.Cleanup(rt.Close)
	return rt
}

func newOKServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// expectReached is a script that asserts the request got the "ok" body.
func expectReached(target string) string {
	return `
		local resp, err = http.get("` + target + `")
		assert(err == nil, "unexpected error: " .. tostring(err and err.message))
		assert(resp.body == "ok", "body = " .. tostring(resp.body))
	`
}

// expectBlocked is a script that asserts the request was refused at dial,
// and that the message names no address.
func expectBlocked(target string) string {
	return `
		local resp, err = http.get("` + target + `")
		assert(resp == nil, "request was not refused")
		assert(err.kind == "blocked", "kind = " .. tostring(err.kind) .. ": " .. tostring(err.message))
		assert(err.message == "http: destination address not allowed", "message = " .. err.message)
		assert(err.details == err.message, "details = " .. err.details)
	`
}

func TestLuaHTTP_Policy(t *testing.T) {
	t.Parallel()
	srv := newOKServer(t)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port := u.Port()
	// localhost resolves to a loopback address, so the strict cases show the
	// check runs on the resolved address, not on the name.
	byName := "http://localhost:" + port + "/"

	tests := []struct {
		name   string
		policy HTTPPolicy
		script string
	}{
		{"default reaches loopback", HTTPPolicy{}, expectReached(srv.URL)},
		{"default reaches loopback by name", HTTPPolicy{}, expectReached(byName)},
		{"default refuses metadata", HTTPPolicy{}, expectBlocked("http://169.254.169.254/latest/meta-data/")},
		{"default refuses v4-mapped metadata", HTTPPolicy{}, expectBlocked("http://[::ffff:169.254.169.254]/")},
		{"default refuses unspecified", HTTPPolicy{}, expectBlocked("http://0.0.0.0:" + port + "/")},
		{"strict refuses loopback", HTTPPolicy{Strict: true}, expectBlocked(srv.URL)},
		{"strict refuses a name resolving to loopback", HTTPPolicy{Strict: true}, expectBlocked(byName)},
		{
			"strict allow prefix reaches loopback",
			HTTPPolicy{Strict: true, Allow: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}},
			expectReached(srv.URL),
		},
		{
			"strict allow host reaches loopback by name",
			HTTPPolicy{Strict: true, AllowHosts: []string{"LOCALHOST:" + port}},
			expectReached(byName),
		},
		{
			"strict allow host on another port does not apply",
			HTTPPolicy{Strict: true, AllowHosts: []string{"localhost:1"}},
			expectBlocked(byName),
		},
		{
			"allow does not reopen metadata",
			HTTPPolicy{Strict: true, Allow: []netip.Prefix{netip.MustParsePrefix("169.254.0.0/16")}},
			expectBlocked("http://169.254.169.254/"),
		},
		{
			"allow host does not reopen metadata",
			HTTPPolicy{Strict: true, AllowHosts: []string{"169.254.169.254:80"}},
			expectBlocked("http://169.254.169.254/"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rt := newPolicyRuntime(t, tc.policy)
			if err := rt.RunString(tc.script); err != nil {
				t.Fatalf("RunString: %v", err)
			}
		})
	}
}

// TestLuaHTTP_PolicyRefusesBeforeConnect pins that the check runs before
// the socket connects: a strict-mode request to a listening loopback port
// must never be accepted by it.
func TestLuaHTTP_PolicyRefusesBeforeConnect(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	accepted := make(chan struct{}, 1)
	go func() {
		c, aerr := ln.Accept()
		if aerr == nil {
			accepted <- struct{}{}
			_ = c.Close()
		}
	}()
	rt := newPolicyRuntime(t, HTTPPolicy{Strict: true})
	if err := rt.RunString(expectBlocked("http://" + ln.Addr().String() + "/")); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	select {
	case <-accepted:
		t.Fatal("the listener accepted a connection the policy refused")
	default:
	}
}

func TestClassifyHTTPError_BlockedHidesAddress(t *testing.T) {
	t.Parallel()
	inner := &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.9.8.7"), Port: 80},
		Err: errDestinationNotAllowed}
	got := classifyHTTPError(&url.Error{Op: "Get", URL: "http://internal.example/", Err: inner})
	if got.Kind != "blocked" {
		t.Fatalf("kind = %q", got.Kind)
	}
	if strings.Contains(got.Message+got.Cause.Error(), "10.9.8.7") {
		t.Fatalf("error leaks the resolved address: %q / %q", got.Message, got.Cause)
	}
}

func TestSetHTTPPolicy_Once(t *testing.T) {
	// Not parallel: it swaps the process-wide client.
	prevClient := sharedHTTPClient.Load()
	prevSet := httpPolicySet.Load()
	t.Cleanup(func() {
		sharedHTTPClient.Store(prevClient)
		httpPolicySet.Store(prevSet)
	})
	httpPolicySet.Store(false)

	if err := SetHTTPPolicy(HTTPPolicy{AllowHosts: []string{"bad"}}); err == nil {
		t.Fatal("SetHTTPPolicy accepted a bad policy")
	}
	if httpPolicySet.Load() {
		t.Fatal("a rejected policy counted as set")
	}
	if err := SetHTTPPolicy(HTTPPolicy{Strict: true}); err != nil {
		t.Fatalf("SetHTTPPolicy: %v", err)
	}
	if sharedHTTPClient.Load() == prevClient {
		t.Fatal("the shared client was not replaced")
	}
	if err := SetHTTPPolicy(HTTPPolicy{}); err == nil {
		t.Fatal("a second SetHTTPPolicy succeeded")
	}
}
