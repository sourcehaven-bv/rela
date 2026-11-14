package main

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
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
		{name: "credentials in the address", flags: withAPI("https://u:p@idp:8091", true), secret: "s",
			wantErr: "-tenant-idp-org-api"},
		{name: "plain http to a remote host", flags: withAPI("http://idp:8091", true), secret: "s",
			wantErr: "needs https"},
		{name: "https to a remote host", flags: withAPI("https://idp:8091", true), secret: "s"},
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

// TestRefuseTenantOnlyFlags pins that the flags only tenant mode acts on are
// refused without it.
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
	if err := refuseTenantOnlyFlags(tenantFlags{provision: "off"}); err != nil {
		t.Fatalf("-tenant-provision=off without tenant mode = %v", err)
	}
	for _, tf := range []tenantFlags{
		{luaHTTPAllow: "10.0.0.0/8"}, {sharedHostConfig: true}, {provision: "lazy"}, {provisionRate: 5},
	} {
		if err := refuseTenantOnlyFlags(tf); err == nil || !strings.Contains(err.Error(), "needs tenant mode") {
			t.Errorf("%+v without tenant mode = %v; want a refusal", tf, err)
		}
		tf.enabled = true
		if err := refuseTenantOnlyFlags(tf); err != nil {
			t.Errorf("%+v in tenant mode = %v", tf, err)
		}
	}
}

// TestTenantProfile pins that the flags map onto the shared tenant
// profile: a strict Lua HTTP policy with the -lua-http-allow destinations,
// and the host config withheld unless -tenant-shared-host-config is set.
// appbuild's tests pin that applying the policy fails closed.
func TestTenantProfile(t *testing.T) {
	p, err := luaHTTPPolicy(tenantFlags{enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Strict || len(p.Allow) != 0 || len(p.AllowHosts) != 0 {
		t.Fatalf("policy %+v; want strict with no allowlist", p)
	}

	p, err = luaHTTPPolicy(tenantFlags{enabled: true, luaHTTPAllow: "10.1.0.0/16,idp.internal:8443"})
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{netip.MustParsePrefix("10.1.0.0/16")}
	if !p.Strict || !slices.Equal(p.Allow, want) || !slices.Equal(p.AllowHosts, []string{"idp.internal:8443"}) {
		t.Fatalf("policy %+v; want strict with the allowlist", p)
	}

	if _, err = luaHTTPPolicy(tenantFlags{enabled: true, luaHTTPAllow: "nope"}); err == nil ||
		!strings.Contains(err.Error(), "-lua-http-allow") {

		t.Errorf("bad allowlist = %v; want an error naming -lua-http-allow", err)
	}

	prof := tenantProfile(tenantFlags{enabled: true})
	if prof.SharedHostConfig || prof.WithheldReason != withheldHostConfigReason || len(prof.Options()) != 1 {
		t.Errorf("profile %+v; want the host config withheld", prof)
	}
	if opts := tenantProfile(tenantFlags{enabled: true, sharedHostConfig: true}).Options(); len(opts) != 0 {
		t.Errorf("shared host config: %d options; want none", len(opts))
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

// TestProvisionConfig pins the flag parsing. tenanthost.New validates the
// rest of the config.
func TestProvisionConfig(t *testing.T) {
	if _, err := provisionConfig(tenantFlags{provision: "always"}, nil); err == nil ||
		!strings.Contains(err.Error(), "-tenant-provision") {

		t.Errorf("unknown mode = %v; want a -tenant-provision error", err)
	}
	for _, v := range []string{"", "off"} {
		cfg, err := provisionConfig(tenantFlags{provision: v}, nil)
		if err != nil || cfg.Mode != tenanthost.ProvisionOff || cfg.Provisioner != nil {
			t.Errorf("%q = %+v, %v; want off", v, cfg, err)
		}
	}
	cfg, err := provisionConfig(tenantFlags{provision: "lazy", provisionRate: 3}, fakeCreator{})
	if err != nil || cfg.Mode != tenanthost.ProvisionLazy || cfg.PerMinute != 3 || cfg.Provisioner == nil {
		t.Errorf("lazy = %+v, %v", cfg, err)
	}
}

type fakeCreator struct{ err error }

func (c fakeCreator) Create(context.Context, string, tenant.CreateOptions) (tenant.Tenant, error) {
	return tenant.Tenant{}, c.err
}

// TestProvisionAdapter pins how a create's error reaches the host: an
// active tenant is success, another status stays an ExistsError, and a
// refusal only an operator resolves is marked ErrNeedsOperator.
func TestProvisionAdapter(t *testing.T) {
	transient := errors.New("connect: refused")
	tests := []struct {
		name          string
		err           error
		wantNil       bool
		wantExists    bool
		needsOperator bool
	}{
		{name: "created", wantNil: true},
		{name: "exists and active", wantNil: true,
			err: &tenant.ExistsError{OrgID: "org-a", Status: tenant.StatusActive}},
		{name: "exists and suspended", wantExists: true,
			err: &tenant.ExistsError{OrgID: "org-a", Status: tenant.StatusSuspended}},
		{name: "schema exists", needsOperator: true, err: fmt.Errorf("x: %w", tenant.ErrSchemaExists)},
		{name: "schema collision", needsOperator: true, err: fmt.Errorf("x: %w", tenant.ErrSchemaCollision)},
		{name: "erased", needsOperator: true, err: fmt.Errorf("x: %w", tenant.ErrTenantErased)},
		{name: "wrong status", needsOperator: true, err: fmt.Errorf("x: %w", tenant.ErrWrongStatus)},
		{name: "transient", err: transient},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := provisionAdapter{fakeCreator{err: tc.err}}.Provision(t.Context(), "org-a")
			if tc.wantNil != (err == nil) {
				t.Fatalf("err = %v; want nil %v", err, tc.wantNil)
			}
			if got := errors.Is(err, tenanthost.ErrNeedsOperator); got != tc.needsOperator {
				t.Errorf("needs operator = %v, want %v (%v)", got, tc.needsOperator, err)
			}
			if got := errors.Is(err, tenant.ErrTenantExists); got != tc.wantExists {
				t.Errorf("exists = %v, want %v (%v)", got, tc.wantExists, err)
			}
			if tc.err != nil && !tc.wantNil && !errors.Is(err, tc.err) {
				t.Errorf("err = %v does not wrap the cause %v", err, tc.err)
			}
		})
	}
}
