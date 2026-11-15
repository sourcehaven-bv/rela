package appbuild

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

func TestTenantProfile_LuaHTTPPolicy(t *testing.T) {
	t.Parallel()
	p, err := TenantProfile{LuaHTTPAllow: " 10.1.0.0/16, ,idp.internal:8443"}.LuaHTTPPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if !p.Strict || !slices.Equal(p.Allow, []netip.Prefix{netip.MustParsePrefix("10.1.0.0/16")}) ||
		!slices.Equal(p.AllowHosts, []string{"idp.internal:8443"}) {

		t.Fatalf("policy = %+v", p)
	}
	if p, err = (TenantProfile{}).LuaHTTPPolicy(); err != nil || !p.Strict || p.Allow != nil || p.AllowHosts != nil {
		t.Fatalf("empty allowlist: %+v, %v", p, err)
	}
	if _, err = (TenantProfile{LuaHTTPAllow: "not a destination"}).LuaHTTPPolicy(); err == nil {
		t.Fatal("a bad allowlist entry parsed")
	}
}

func TestTenantProfile_Options(t *testing.T) {
	t.Parallel()
	if opts := (TenantProfile{SharedHostConfig: true}).Options(); len(opts) != 0 {
		t.Fatalf("shared host config: %d options, want none", len(opts))
	}
	opts := TenantProfile{WithheldReason: "not shared here"}.Options()
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.withheld != "not shared here" {
		t.Fatalf("withheld = %q", o.withheld)
	}
}

// TestTenantProfile_ApplyFailsClosed pins that applying the profile fails
// unless a strict policy is installed afterwards.
func TestTenantProfile_ApplyFailsClosed(t *testing.T) {
	t.Parallel()
	var installed *lua.HTTPPolicy
	set := func(p lua.HTTPPolicy) error { installed = &p; return nil }
	current := func() (lua.HTTPPolicy, bool) {
		if installed == nil {
			return lua.HTTPPolicy{}, false
		}
		return *installed, true
	}
	prof := TenantProfile{LuaHTTPAllow: "10.1.0.0/16"}
	p, err := prof.applyLuaHTTPPolicy(set, current)
	if err != nil || !p.Strict || installed == nil || !installed.Strict {
		t.Fatalf("apply = %+v, %v; installed %+v", p, err, installed)
	}

	tests := []struct {
		name    string
		prof    TenantProfile
		set     func(lua.HTTPPolicy) error
		current func() (lua.HTTPPolicy, bool)
		wantErr string
	}{
		{
			"bad allowlist", TenantProfile{LuaHTTPAllow: "nope"},
			func(lua.HTTPPolicy) error { return errors.New("set called") }, current, "allowlist",
		},
		{
			"set fails", prof, func(lua.HTTPPolicy) error { return errors.New("already set") }, current,
			"set the Lua HTTP policy",
		},
		{
			"nothing installed", prof, func(lua.HTTPPolicy) error { return nil },
			func() (lua.HTTPPolicy, bool) { return lua.HTTPPolicy{}, false }, "not installed",
		},
		{
			"default installed", prof, func(lua.HTTPPolicy) error { return nil },
			func() (lua.HTTPPolicy, bool) { return lua.HTTPPolicy{}, true }, "not installed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tc.prof.applyLuaHTTPPolicy(tc.set, tc.current)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}
