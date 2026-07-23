package main

import "testing"

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
