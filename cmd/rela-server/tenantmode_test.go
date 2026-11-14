package main

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// TestValidateTenantMode pins the boot refusals of tenant mode. Each would
// otherwise run a server that cannot pick a tenant safely.
func TestValidateTenantMode(t *testing.T) {
	tests := []struct {
		name      string
		flags     func() *serverFlags
		envUser   string
		supported bool
		wantErr   string
	}{
		{name: "jwt on the postgres build", flags: jwtFlags, supported: true},
		{name: "build without a DSN store", flags: jwtFlags, supported: false, wantErr: "postgres build"},
		{name: "no identity source", flags: func() *serverFlags { return &serverFlags{} }, supported: true,
			wantErr: "needs JWT identity"},
		{name: "principal header only", supported: true, wantErr: "needs JWT identity",
			flags: func() *serverFlags { return &serverFlags{principalHeader: "X-Forwarded-User"} }},
		{name: "env user only", flags: func() *serverFlags { return &serverFlags{} }, envUser: "jeroen",
			supported: true, wantErr: "needs JWT identity"},
		{name: "jwt with principal header", supported: true, wantErr: "mutually exclusive",
			flags: func() *serverFlags { f := jwtFlags(); f.principalHeader = "X-Forwarded-User"; return f }},
		{name: "jwt with env user", flags: jwtFlags, envUser: "jeroen", supported: true, wantErr: "cannot be set"},
		{name: "config editing", supported: true, wantErr: "--config-editing",
			flags: func() *serverFlags { f := jwtFlags(); f.configEditing = true; return f }},
		{name: "provision lazy without webhook", supported: true,
			flags: func() *serverFlags { f := jwtFlags(); f.tenant.provision = "lazy"; return f }},
		{name: "provision off", supported: true,
			flags: func() *serverFlags { f := jwtFlags(); f.tenant.provision = "off"; return f }},
		{name: "provision webhook with webhook", supported: true, flags: func() *serverFlags {
			f := jwtFlags()
			f.tenant.provision, f.webhookAudience, f.webhookAction = "webhook", "rela-webhook", "idp-sync"
			return f
		}},
		{name: "provision webhook without webhook", supported: true, wantErr: "needs the IdP webhook",
			flags: func() *serverFlags { f := jwtFlags(); f.tenant.provision = "webhook"; return f }},
		{name: "provision unknown mode", supported: true, wantErr: "-tenant-provision",
			flags: func() *serverFlags { f := jwtFlags(); f.tenant.provision = "always"; return f }},
		{name: "provision negative rate", supported: true, wantErr: "-tenant-provision-rate",
			flags: func() *serverFlags { f := jwtFlags(); f.tenant.provisionRate = -1; return f }},
		{name: "lua http allowlist", supported: true, flags: func() *serverFlags {
			f := jwtFlags()
			f.tenant.luaHTTPAllow = "10.1.0.0/16, 10.2.3.4,idp.internal:8443"
			return f
		}},
		{name: "lua http allowlist unparsable", supported: true, wantErr: "-lua-http-allow",
			flags: func() *serverFlags { f := jwtFlags(); f.tenant.luaHTTPAllow = "idp.internal"; return f }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mode, err := validateTenantMode(tc.flags(), tc.envUser, tc.supported)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if mode != identityJWT {
					t.Errorf("mode = %v, want identityJWT", mode)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

// TestTenantModeBackend pins that only the postgres build supports tenant
// mode. The default build runs this test, so it asserts the refusal.
func TestTenantModeBackend(t *testing.T) {
	if tenant.Supported {
		t.Skip("postgres build")
	}
	if _, err := validateTenantMode(jwtFlags(), "", tenant.Supported); err == nil {
		t.Fatal("a build without a DSN store accepted tenant mode")
	}
}

// TestValidateStatusSync pins the boot refusals of IdP status sync.
func TestValidateStatusSync(t *testing.T) {
	withAPI := func(api string, webhook bool) *serverFlags {
		f := jwtFlags()
		f.tenant.enabled, f.tenant.idpOrgAPI = true, api
		if webhook {
			f.webhookAudience, f.webhookAction = "rela-webhook", "idp-sync"
		}
		return f
	}
	tests := []struct {
		name    string
		flags   *serverFlags
		secret  string
		wantErr string
	}{
		{name: "off", flags: jwtFlags()},
		{name: "http with webhook and secret", flags: withAPI("http://127.0.0.1:8091", true), secret: "s"},
		{name: "unix socket", flags: withAPI("unix:///run/pratique/op.sock", true), secret: "s"},
		{name: "no webhook", flags: withAPI("http://127.0.0.1:8091", false), secret: "s",
			wantErr: "needs the IdP webhook"},
		{name: "no secret", flags: withAPI("http://127.0.0.1:8091", true), wantErr: envIDPOperatorSecret},
		{name: "bad address", flags: withAPI("ftp://idp", true), secret: "s", wantErr: "-tenant-idp-org-api"},
		{name: "credentials in the address", flags: withAPI("http://u:p@idp:8091", true), secret: "s",
			wantErr: "-tenant-idp-org-api"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStatusSync(tc.flags, tc.secret)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

// TestRefuseTenantOnlyFlags pins that the IdP org API is refused without
// tenant mode.
func TestRefuseTenantOnlyFlags(t *testing.T) {
	if err := refuseTenantOnlyFlags(tenantFlags{idpOrgAPI: "http://idp:8091"}); err == nil ||
		!strings.Contains(err.Error(), "needs tenant mode") {

		t.Fatalf("org API without tenant mode = %v; want a refusal", err)
	}
	if err := refuseTenantOnlyFlags(tenantFlags{enabled: true, idpOrgAPI: "http://idp:8091"}); err != nil {
		t.Fatalf("org API in tenant mode = %v", err)
	}
	if err := refuseTenantOnlyFlags(tenantFlags{}); err != nil {
		t.Fatalf("no flags = %v", err)
	}
	for _, tf := range []tenantFlags{{luaHTTPAllow: "10.0.0.0/8"}, {sharedHostConfig: true}} {
		if err := refuseTenantOnlyFlags(tf); err == nil || !strings.Contains(err.Error(), "needs tenant mode") {
			t.Errorf("%+v without tenant mode = %v; want a refusal", tf, err)
		}
		tf.enabled = true
		if err := refuseTenantOnlyFlags(tf); err != nil {
			t.Errorf("%+v in tenant mode = %v", tf, err)
		}
	}
}

// TestApplyLuaHTTPPolicy pins that tenant mode sets a strict Lua HTTP
// policy with the -lua-http-allow destinations, and that a failure to set
// it stops startup.
func TestApplyLuaHTTPPolicy(t *testing.T) {
	var got []lua.HTTPPolicy
	record := func(p lua.HTTPPolicy) error { got = append(got, p); return nil }

	p, err := applyLuaHTTPPolicy(tenantFlags{enabled: true}, record)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Strict || len(got[0].Allow) != 0 || len(got[0].AllowHosts) != 0 {
		t.Fatalf("set %+v; want one strict policy with no allowlist", got)
	}
	if !p.Strict {
		t.Error("returned policy is not strict")
	}

	got = nil
	if _, err = applyLuaHTTPPolicy(tenantFlags{enabled: true, luaHTTPAllow: "10.1.0.0/16,idp.internal:8443"},
		record); err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{netip.MustParsePrefix("10.1.0.0/16")}
	if len(got) != 1 || !got[0].Strict || !slices.Equal(got[0].Allow, want) ||
		!slices.Equal(got[0].AllowHosts, []string{"idp.internal:8443"}) {

		t.Fatalf("set %+v; want strict with the allowlist", got)
	}

	if _, err = applyLuaHTTPPolicy(tenantFlags{enabled: true},
		func(lua.HTTPPolicy) error { return errors.New("already set") }); err == nil {
		t.Error("a failed SetHTTPPolicy did not stop startup")
	}
}

// TestBuildCommandAuthorizer_TenantMode pins that tenant mode installs the
// authorizer that refuses every command, whatever the ACL and bind.
func TestBuildCommandAuthorizer_TenantMode(t *testing.T) {
	f := &serverFlags{bind: "127.0.0.1", allowUnauthCommands: true}
	f.tenant.enabled = true
	authz, err := buildCommandAuthorizer(f, nil)
	if err != nil {
		t.Fatal(err)
	}
	if authz != dataentry.TenantModeCommandAuthorizer() {
		t.Fatalf("authorizer = %T; want the tenant-mode one", authz)
	}
	if authz.Authorize(context.Background(), dataentry.CommandConfig{Script: "true"}) {
		t.Error("tenant mode authorized a command")
	}
}
