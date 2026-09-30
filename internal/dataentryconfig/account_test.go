package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// accountMeta is testMetamodel with a `file` photo property on ticket.
func accountMeta() *metamodel.Metamodel {
	m := testMetamodel()
	ticket := m.Entities["ticket"]
	ticket.Properties["photo"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeFile}
	m.Entities["ticket"] = ticket
	return m
}

func TestValidateConfig_Account(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr string // substring; "" means expect success
	}{
		{"absent", ``, ""},
		{"empty block", "account: {}", ""},
		{"full pratique block", `account:
  sign_out: /pratique/auth/logout
  account: /pratique/admin/account
  switch_org: /pratique/auth/select-tenant
  admin: { url: /pratique/admin/, role: org-admin }
  avatar_property: photo`, ""},
		{"https links", `account:
  sign_out: https://auth.example.com/oauth2/sign_out
  admin: { url: "https://auth.example.com/admin?x=1" }`, ""},
		{"root path", "account: {sign_out: /}", ""},
		{"string avatar property", "account: {avatar_property: assignee}", ""},

		{"javascript scheme", "account: {sign_out: 'javascript:alert(1)'}", `account.sign_out: "javascript:alert(1)"`},
		{"data scheme", "account: {account: 'data:text/html,x'}", "account.account:"},
		{"http scheme", "account: {switch_org: 'http://auth.example.com/'}", "account.switch_org:"},
		{"uppercase javascript", "account: {sign_out: 'JAVASCRIPT:alert(1)'}", "account.sign_out:"},
		{"protocol-relative", "account: {sign_out: //evil.example/logout}", "not a protocol-relative URL"},
		{"backslash protocol-relative", `account: {sign_out: '/\evil.example'}`, "not a protocol-relative URL"},
		{"relative without slash", "account: {sign_out: auth/logout}", "must be a path starting with /"},
		{"https without host", "account: {sign_out: 'https:///x'}", "account.sign_out:"},
		{"https with user info", "account: {sign_out: 'https://u:p@auth.example.com/'}", "account.sign_out:"},
		{"whitespace", "account: {sign_out: '/a b'}", "whitespace"},
		{"admin url invalid", "account: {admin: {url: 'javascript:x'}}", "account.admin.url:"},
		{"admin without url", "account: {admin: {role: org-admin}}", "account.admin.url: required"},

		{"unknown key", "account: {signout: /logout}", `account: unknown key "signout"`},
		{"unknown admin key", "account: {admin: {url: /admin/, roles: [a]}}", `account.admin: unknown key "roles"`},

		{"avatar property undeclared", "account: {avatar_property: selfie}", `no entity type declares a property "selfie"`},
		{"avatar property wrong type", "account: {avatar_property: status}", `must be file or string`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte("version: \"1.0\"\n" + tc.yaml)
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			err := ValidateConfig(data, &cfg, accountMeta())
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidAccountLink(t *testing.T) {
	for _, tc := range []struct {
		link string
		want bool
	}{
		{"/", true},
		{"/pratique/auth/logout", true},
		{"/.pomerium/sign_out", true},
		{"https://auth.example.com/oauth2/sign_out?rd=/", true},
		{"", false},
		{"//evil.example", false},
		{`/\evil.example`, false},
		{"javascript:alert(1)", false},
		{"http://auth.example.com/", false},
		{"logout", false},
		{"/a\nb", false},
		{"ftp://x/", false},
	} {
		if got := ValidAccountLink(tc.link); got != tc.want {
			t.Errorf("ValidAccountLink(%q) = %v, want %v", tc.link, got, tc.want)
		}
	}
}
