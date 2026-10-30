package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/jwtauth"
)

// jwtFlags returns a serverFlags with JWT identity fully configured, so each
// test case can express exactly one deviation from a valid config.
func jwtFlags() *serverFlags {
	return &serverFlags{
		jwtIssuer:   "https://idp.example",
		jwtAudience: "rela",
		jwtJWKSURL:  "https://idp.example/jwks",
		jwtHeader:   "X-Auth-Assertion",
	}
}

// validateIdentityFlags is the startup guard that makes JWT identity exclusive.
// The combinations it refuses are the ones where a JWT verification failure
// would otherwise fall through to a spoofable source — the downgrade this whole
// change exists to close.
func TestValidateIdentityFlags(t *testing.T) {
	tests := []struct {
		name    string
		flags   func() *serverFlags
		envUser string
		want    identityMode
		wantErr bool
	}{
		{
			name:  "nothing configured → header mode (unchanged legacy default)",
			flags: func() *serverFlags { return &serverFlags{} },
			want:  identityHeader,
		},
		{
			name:  "header only → header mode",
			flags: func() *serverFlags { return &serverFlags{principalHeader: "X-Forwarded-User"} },
			want:  identityHeader,
		},
		{
			name:    "env user only → header mode (local-dev hatch still works)",
			flags:   func() *serverFlags { return &serverFlags{} },
			envUser: "jeroen",
			want:    identityHeader,
		},
		{
			name:  "all three jwt flags → jwt mode",
			flags: jwtFlags,
			want:  identityJWT,
		},
		{
			name: "jwt + principal-header → error (the downgrade path)",
			flags: func() *serverFlags {
				f := jwtFlags()
				f.principalHeader = "X-Forwarded-User"
				return f
			},
			wantErr: true,
		},
		{
			name:    "jwt + $RELA_DATAENTRY_USER → error (env would override a verified subject)",
			flags:   jwtFlags,
			envUser: "jeroen",
			wantErr: true,
		},
		{
			name: "jwt + both → error",
			flags: func() *serverFlags {
				f := jwtFlags()
				f.principalHeader = "X-Forwarded-User"
				return f
			},
			envUser: "jeroen",
			wantErr: true,
		},
		{
			name: "issuer only → error (partial config used to silently disable identity)",
			flags: func() *serverFlags {
				return &serverFlags{jwtIssuer: "https://idp.example"}
			},
			wantErr: true,
		},
		{
			name: "missing jwks url → error",
			flags: func() *serverFlags {
				f := jwtFlags()
				f.jwtJWKSURL = ""
				return f
			},
			wantErr: true,
		},
		{
			name: "missing audience → error (the confused-deputy guard)",
			flags: func() *serverFlags {
				f := jwtFlags()
				f.jwtAudience = ""
				return f
			},
			wantErr: true,
		},
		{
			// An empty header name reads every assertion as absent, so the server
			// would boot clean and then deny every API request.
			name: "empty jwt header → error",
			flags: func() *serverFlags {
				f := jwtFlags()
				f.jwtHeader = ""
				return f
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateIdentityFlags(tt.flags(), tt.envUser)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got mode %v and nil error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("mode = %v, want %v", got, tt.want)
			}
		})
	}
}

// assertedIdentity must copy every claim. A forgotten field fails silently:
// a dropped role removes access, a dropped principal_type removes a ceiling
// (TKT-OJL2GN, TKT-IAC8TX), and a dropped expiry keeps streams open past it
// (TKT-5I6REX). Every input field gets a value unique to it, and each output
// field must equal the input field of the same name, so a dropped field and
// a swapped one both fail.
func TestAssertedIdentity_CopiesEveryField(t *testing.T) {
	var claims jwtauth.AssertionClaims
	cv := reflect.ValueOf(&claims).Elem()
	for i := range cv.NumField() {
		f, name := cv.Field(i), cv.Type().Field(i).Name
		switch {
		case f.Kind() == reflect.String:
			f.SetString(name)
		case f.Type() == reflect.TypeFor[[]string]():
			f.Set(reflect.ValueOf([]string{name}))
		case f.Type() == reflect.TypeFor[time.Time]():
			f.Set(reflect.ValueOf(time.Unix(int64(i+1)*1000, 0)))
		default:
			t.Fatalf("AssertionClaims.%s: no test value for %s", name, f.Type())
		}
	}

	got := reflect.ValueOf(assertedIdentity(claims))
	for i := range got.NumField() {
		name := got.Type().Field(i).Name
		want := cv.FieldByName(name)
		if !want.IsValid() {
			t.Errorf("AssertedIdentity.%s has no AssertionClaims field of the same name", name)
			continue
		}
		if !reflect.DeepEqual(got.Field(i).Interface(), want.Interface()) {
			t.Errorf("AssertedIdentity.%s = %v, want %v", name, got.Field(i), want)
		}
	}
}

// TestWarnUnidentifiedACL pins when startup warns that every /api/ request
// will be refused with 401 unauthenticated (TKT-QR1VTP).
func TestWarnUnidentifiedACL(t *testing.T) {
	tests := []struct {
		name    string
		mode    identityMode
		header  string
		envUser string
		hasACL  bool
		want    bool
	}{
		{name: "acl without identity source", mode: identityHeader, hasACL: true, want: true},
		{name: "no acl", mode: identityHeader},
		{name: "principal header", mode: identityHeader, header: "X-Forwarded-User", hasACL: true},
		{name: "env user", mode: identityHeader, envUser: "alice", hasACL: true},
		{name: "jwt identity", mode: identityJWT, hasACL: true},
		{name: "jwt without acl", mode: identityJWT},
		{name: "blank env user", mode: identityHeader, envUser: "  ", hasACL: true, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &serverFlags{principalHeader: tc.header}
			if got := warnUnidentifiedACL(tc.mode, f, tc.envUser, tc.hasACL); got != tc.want {
				t.Errorf("warnUnidentifiedACL() = %v, want %v", got, tc.want)
			}
		})
	}
}
