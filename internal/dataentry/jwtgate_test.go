package dataentry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// errStubKeysUnavailable stands in for jwtauth.ErrKeysUnavailable. The gate
// classifies via an injected predicate rather than importing jwtauth, so the
// test supplies its own sentinel — which is exactly the seam that keeps
// dataentry independent of the verifier package.
var errStubKeysUnavailable = errors.New("keys unavailable")

// gateVerifier returns a fixed subject for a known token, and lets a test choose
// which error a bad token produces so the keys-unavailable branch is reachable.
type gateVerifier struct {
	validToken string
	subject    string
	orgID      string
	orgSlug    string
	roles      []string
	failWith   error
}

func (g gateVerifier) VerifyAssertion(_ context.Context, raw string) (AssertedIdentity, error) {
	if raw == g.validToken {
		return AssertedIdentity{
			Subject: g.subject,
			OrgID:   g.orgID,
			OrgSlug: g.orgSlug,
			Roles:   g.roles,
		}, nil
	}
	if g.failWith != nil {
		return AssertedIdentity{}, g.failWith
	}
	return AssertedIdentity{}, errors.New("invalid")
}

// newGateHandler builds the gate over a handler that echoes the stamped
// principal, mirroring the production wrap order: the stamper runs first, then
// the gate overwrites on API paths.
func newGateHandler(t *testing.T, cfg JWTGateConfig) http.Handler {
	t.Helper()
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, principal.From(r.Context()).User)
	})
	return stampAuditPrincipal(requireVerifiedJWT(echo, cfg), defaultPrincipalResolver)
}

// TestRequireVerifiedJWT covers the gate's whole decision table: which paths are
// gated, and what each class of assertion outcome yields.
func TestRequireVerifiedJWT(t *testing.T) {
	const header = "X-Auth-Assertion"

	tests := []struct {
		name       string
		path       string
		token      string // "" ⇒ send no header at all
		failWith   error
		subject    string
		wantStatus int
		wantUser   string // only checked on 200
	}{
		{
			name:       "valid assertion on api path → 200 with verified subject",
			path:       "/api/v1/entities",
			token:      "good.jwt.token",
			wantStatus: http.StatusOK,
			wantUser:   "usr_abc123",
		},
		{
			name:       "absent assertion on api path → 401",
			path:       "/api/v1/entities",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid assertion on api path → 401",
			path:       "/api/v1/entities",
			token:      "forged.jwt.token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "keys unavailable → 401 (fails closed, same as invalid)",
			path:       "/api/v1/entities",
			token:      "unknown.kid.token",
			failWith:   errStubKeysUnavailable,
			wantStatus: http.StatusUnauthorized,
		},
		{
			// RR-P2M7: Go's ServeMux won't match the bare /api under the /api/
			// pattern, so it needs an explicit case or it bypasses the gate.
			name:       "bare /api → 401 (RR-P2M7)",
			path:       "/api",
			wantStatus: http.StatusUnauthorized,
		},
		{
			// SSE is registered on the OUTER mux and bypasses noCacheMiddleware.
			// It streams live entity data, so an ungated SSE endpoint would be
			// the worst possible miss; nothing else in the suite pins this.
			name:       "SSE /api/events → 401",
			path:       "/api/events",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "SSE /api/v1/_events → 401",
			path:       "/api/v1/_events",
			wantStatus: http.StatusUnauthorized,
		},
		{
			// RR-T15E: the SPA shell must stay reachable so a client can load and
			// render a signed-out state, and so a misconfiguration doesn't lock
			// operators out of the recovery surface.
			name:       "SPA shell / → 200 ungated",
			path:       "/",
			wantStatus: http.StatusOK,
			wantUser:   "unknown",
		},
		{
			name:       "static asset → 200 ungated",
			path:       "/static/app.js",
			wantStatus: http.StatusOK,
			wantUser:   "unknown",
		},
		{
			// The IdP webhook verifies a signed body against its OWN audience and
			// will never carry an identity assertion; gating it would reject every
			// legitimate callback.
			name:       "webhook path → 200 ungated",
			path:       "/webhooks/idp",
			wantStatus: http.StatusOK,
			wantUser:   "unknown",
		},
		{
			name:       "control-char-only subject sanitizes to empty → 401",
			path:       "/api/v1/entities",
			token:      "good.jwt.token",
			subject:    "\x00\x01\x02",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject := tt.subject
			if subject == "" {
				subject = "usr_abc123"
			}
			h := newGateHandler(t, JWTGateConfig{
				Verifier: gateVerifier{
					validToken: "good.jwt.token",
					subject:    subject,
					failWith:   tt.failWith,
				},
				HeaderName:      header,
				KeysUnavailable: func(err error) bool { return errors.Is(err, errStubKeysUnavailable) },
			})

			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			if tt.token != "" {
				req.Header.Set(header, tt.token)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusOK && rec.Body.String() != tt.wantUser {
				t.Errorf("principal user = %q, want %q", rec.Body.String(), tt.wantUser)
			}
		})
	}
}

// A denial must not explain itself: the assertion is attacker-controlled input,
// so echoing why it failed turns the endpoint into a verification oracle.
func TestRequireVerifiedJWT_DenialLeaksNoReason(t *testing.T) {
	h := newGateHandler(t, JWTGateConfig{
		Verifier: gateVerifier{
			validToken: "good.jwt.token",
			subject:    "usr_abc123",
			failWith:   errors.New("token is expired by 3h3m0s: signature kid=abc123 unknown"),
		},
		HeaderName: "X-Auth-Assertion",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set("X-Auth-Assertion", "expired.jwt.token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := strings.ToLower(rec.Body.String())
	for _, leak := range []string{"expired", "signature", "kid", "abc123", "3h3m"} {
		if strings.Contains(body, leak) {
			t.Errorf("401 body leaks verification detail %q: %s", leak, rec.Body.String())
		}
	}
}

// A spoofed proxy-trusted header must not authenticate anything in JWT mode.
// This is the downgrade the change exists to close, expressed end-to-end.
func TestRequireVerifiedJWT_SpoofedHeaderDoesNotAuthenticate(t *testing.T) {
	h := newGateHandler(t, JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: "usr_abc123"},
		HeaderName: "X-Auth-Assertion",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set("X-Forwarded-User", "admin")
	req.Header.Set("X-User", "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("spoofed header got status %d, want 401 — identity downgraded", rec.Code)
	}
}

// WWW-Authenticate is advertised only when the assertion rides the standard
// Authorization header; for a custom proxy header the challenge is meaningless.
func TestRequireVerifiedJWT_WWWAuthenticate(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"authorization header → challenge", "Authorization", "Bearer"},
		{"custom header → no challenge", "X-Auth-Assertion", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newGateHandler(t, JWTGateConfig{
				Verifier:   gateVerifier{validToken: "good.jwt.token", subject: "usr_abc123"},
				HeaderName: tt.header,
			})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody))

			if got := rec.Header().Get("WWW-Authenticate"); got != tt.want {
				t.Errorf("WWW-Authenticate = %q, want %q", got, tt.want)
			}
		})
	}
}

// An "Authorization: Bearer <jwt>" wrapper must be unwrapped, matching
// JWTPrincipalResolver's input handling so the two cannot diverge.
func TestRequireVerifiedJWT_StripsBearerPrefix(t *testing.T) {
	h := newGateHandler(t, JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: "usr_abc123"},
		HeaderName: "Authorization",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set("Authorization", "Bearer good.jwt.token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "usr_abc123" {
		t.Errorf("principal user = %q, want %q", got, "usr_abc123")
	}
}

// A nil KeysUnavailable predicate must classify everything as a client fault
// rather than panicking — the field is documented optional.
func TestRequireVerifiedJWT_NilKeysUnavailablePredicate(t *testing.T) {
	h := newGateHandler(t, JWTGateConfig{
		Verifier: gateVerifier{
			validToken: "good.jwt.token",
			subject:    "usr_abc123",
			failWith:   errStubKeysUnavailable,
		},
		HeaderName: "X-Auth-Assertion",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set("X-Auth-Assertion", "bad.jwt.token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// isAPIPath is shared by the gate and attachACLRequest so the two cannot drift.
func TestIsAPIPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v1/entities", true},
		{"/api/", true},
		{"/api", true}, // RR-P2M7
		{"/api/events", true},
		{"/", false},
		{"/static/app.js", false},
		{"/webhooks/idp", false},
		{"/apixyz", false}, // prefix must not match a longer segment
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isAPIPath(tt.path); got != tt.want {
				t.Errorf("isAPIPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// The gate must sit BETWEEN stampAuditPrincipal and attachACLRequest at wrap
// time, which at request time means: stamper first, then gate, then ACL.
//
// This pins the ordering hazard documented in NewRouter. Both alternatives are
// silently wrong rather than loudly broken:
//   - gate outermost ⇒ the stamper runs LAST and overwrites the verified subject
//     with "unknown", discarding an identity we just proved;
//   - gate innermost ⇒ ACL opens a Request for the unverified principal before
//     the gate can deny.
func TestRequireVerifiedJWT_OrderingRelativeToStamper(t *testing.T) {
	const header = "X-Auth-Assertion"
	cfg := JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: "usr_abc123"},
		HeaderName: header,
	}

	// Production order: gate wrapped first (inner), stamper wrapped last (outer).
	var seen principal.Principal
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = principal.From(r.Context())
	})
	h := stampAuditPrincipal(requireVerifiedJWT(inner, cfg), defaultPrincipalResolver)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set(header, "good.jwt.token")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if seen.User != "usr_abc123" {
		t.Fatalf("downstream principal = %q, want the verified subject %q — "+
			"the stamper overwrote the gate's identity", seen.User, "usr_abc123")
	}

	// The inverted wrap must NOT survive: it is the mistake the comment warns
	// about, so assert it genuinely loses the verified subject. If this ever
	// stops being true the ordering comment is stale and should be revisited.
	seen = principal.Principal{}
	inverted := requireVerifiedJWT(stampAuditPrincipal(inner, defaultPrincipalResolver), cfg)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req2.Header.Set(header, "good.jwt.token")
	inverted.ServeHTTP(httptest.NewRecorder(), req2)

	if seen.User == "usr_abc123" {
		t.Error("inverted wrap preserved the verified subject; the NewRouter " +
			"ordering comment no longer describes a real hazard")
	}
}

// TestRequireVerifiedJWT_StampsAssertedClaims pins that the gate stamps the
// FULL verified identity — org and roles, not just the subject. This is the
// direct regression guard for TKT-OJL2GN: the gate used to stamp a subject-only
// literal, silently dropping every asserted claim before the ACL saw it.
func TestRequireVerifiedJWT_StampsAssertedClaims(t *testing.T) {
	const header = "X-Auth-Assertion"
	cfg := JWTGateConfig{
		Verifier: gateVerifier{
			validToken: "good.jwt.token", subject: "usr_abc123",
			orgID: "org_acme", orgSlug: "acme", roles: []string{"admin", "billing"},
		},
		HeaderName: header,
	}

	var seen principal.Principal
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = principal.From(r.Context())
	})
	h := stampAuditPrincipal(requireVerifiedJWT(inner, cfg), defaultPrincipalResolver)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set(header, "good.jwt.token")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if seen.User != "usr_abc123" {
		t.Fatalf("User = %q, want the verified subject", seen.User)
	}
	if seen.OrgID() != "org_acme" {
		t.Errorf("OrgID = %q, want org_acme — the gate dropped the org claim", seen.OrgID())
	}
	if seen.OrgSlug() != "acme" {
		t.Errorf("OrgSlug = %q, want acme", seen.OrgSlug())
	}
	if want := []string{"admin", "billing"}; !slices.Equal(seen.Roles(), want) {
		t.Errorf("Roles = %v, want %v — the gate dropped the asserted roles", seen.Roles(), want)
	}
}

// TestRequireVerifiedJWT_SanitizesRolesThroughGate drives roles carrying control
// characters through the gate and asserts they are cleaned before landing on the
// Principal. The per-element cap and control-char strip live in the shared
// verifiedPrincipal projection; this pins that they run on the gate path
// specifically, not only in jwtauth's own unit tests. A hostile IdP must not be
// able to inject a control-char role into the audit JSONL stream via the gate.
//
// (The 32-role count cap is enforced in jwtauth.VerifyAssertion, upstream of the
// stub used here, so it is covered by internal/jwtauth; this guards the
// dataentry-side per-element sanitization the gate applies.)
func TestRequireVerifiedJWT_SanitizesRolesThroughGate(t *testing.T) {
	const header = "X-Auth-Assertion"
	cfg := JWTGateConfig{
		Verifier: gateVerifier{
			validToken: "good.jwt.token", subject: "usr_abc123",
			// A control char in a role, and one role that is control-only (must
			// be dropped, since an empty role can never match a policy mapping).
			roles: []string{"ad\x00min", "\x00\x00", "ok"},
		},
		HeaderName: header,
	}

	var seen principal.Principal
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = principal.From(r.Context())
	})
	h := stampAuditPrincipal(requireVerifiedJWT(inner, cfg), defaultPrincipalResolver)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set(header, "good.jwt.token")
	h.ServeHTTP(httptest.NewRecorder(), req)

	for _, role := range seen.Roles() {
		if strings.ContainsRune(role, 0) {
			t.Errorf("role %q reached the Principal with a control char — the gate "+
				"did not sanitize", role)
		}
		if role == "" {
			t.Error("an empty role survived the gate; it can never match a policy " +
				"mapping and only pads the attribution set")
		}
	}
	// "ad\x00min" -> "ad min", "\x00\x00" -> dropped, "ok" -> "ok".
	if len(seen.Roles()) != 2 {
		t.Errorf("Roles = %v, want 2 surviving entries", seen.Roles())
	}
}

// A denied request must never reach the handlers the gate protects — no store
// read, no ACL request, no side effect.
func TestRequireVerifiedJWT_DenialShortCircuits(t *testing.T) {
	reached := 0
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached++ })
	h := stampAuditPrincipal(requireVerifiedJWT(inner, JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: "usr_abc123"},
		HeaderName: "X-Auth-Assertion",
	}), defaultPrincipalResolver)

	for _, token := range []string{"", "forged.jwt.token"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
		if token != "" {
			req.Header.Set("X-Auth-Assertion", token)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	if reached != 0 {
		t.Errorf("protected handler reached %d times on denied requests, want 0", reached)
	}
}

// The operator-intuitive case the spoofing test above does NOT cover: a VALID
// assertion arriving alongside a spoofed proxy header must resolve to the JWT's
// subject, not the header's. This is the property the whole change exists to
// guarantee, so it gets its own pin.
func TestRequireVerifiedJWT_ValidAssertionBeatsSpoofedHeader(t *testing.T) {
	h := newGateHandler(t, JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: "usr_abc123"},
		HeaderName: "X-Auth-Assertion",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set("X-Auth-Assertion", "good.jwt.token")
	req.Header.Set("X-Forwarded-User", "admin")
	req.Header.Set("X-User", "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "usr_abc123" {
		t.Errorf("principal user = %q, want the verified subject %q — a spoofed header won", got, "usr_abc123")
	}
}

// SetJWTGate must reject a config that cannot work, rather than deferring the
// failure to a per-request panic or a silent deny-everything.
func TestSetJWTGate_RejectsIncompleteConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     JWTGateConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     JWTGateConfig{Verifier: gateVerifier{}, HeaderName: "X-Auth-Assertion"},
			wantErr: false,
		},
		{
			name:    "nil verifier → error (would panic per-request)",
			cfg:     JWTGateConfig{HeaderName: "X-Auth-Assertion"},
			wantErr: true,
		},
		{
			name:    "empty header name → error (would deny every API request silently)",
			cfg:     JWTGateConfig{Verifier: gateVerifier{}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&App{}).SetJWTGate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetJWTGate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Both denial classes are sampled independently, and the residual is reported
// when the burst ends — otherwise an outage's tail would vanish from the log.
func TestLogSampler(t *testing.T) {
	t.Run("first occurrence logs, immediate repeats are suppressed", func(t *testing.T) {
		var s logSampler
		if _, ok := s.sample(); !ok {
			t.Fatal("first sample should log")
		}
		for i := range 5 {
			if _, ok := s.sample(); ok {
				t.Fatalf("sample %d within the interval should be suppressed", i)
			}
		}
		if n := s.drain(); n != 5 {
			t.Errorf("drained %d, want the 5 suppressed occurrences", n)
		}
	})

	t.Run("drain resets so a later burst logs again", func(t *testing.T) {
		var s logSampler
		s.sample()
		s.sample()
		s.drain()
		if _, ok := s.sample(); !ok {
			t.Error("after drain, the next sample should log")
		}
	})
}

// Every other gate test hand-composes stampAuditPrincipal(requireVerifiedJWT(...)),
// so a reordering of the wraps in NewRouter would leave them all passing. This
// one asserts the gate through the REAL router composition, with ACL active —
// the same reasoning as TestACLMiddleware_RouterChainOrder, which exists because
// that class of bug only ever appears at the composition site.
func TestJWTGate_RouterChainOrder(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})

	// The verified subject must be the principal ACL authorizes against: usr_alice
	// is a viewer, so a correctly-stamped request reads the ticket.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"usr_alice": "viewer"},
	}, app.store)
	app.acl = d

	if err := app.SetJWTGate(JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: "usr_alice"},
		HeaderName: "X-Auth-Assertion",
	}); err != nil {
		t.Fatalf("SetJWTGate: %v", err)
	}
	handler := app.NewRouter()

	t.Run("valid assertion reaches ACL as the verified subject", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
		req.Header.Set("X-Auth-Assertion", "good.jwt.token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if strings.Contains(rec.Body.String(), "acl_unstamped_principal") {
			t.Fatalf("gate wrapped outside stampAuditPrincipal: ACL saw an unstamped "+
				"principal (got %d %s)", rec.Code, rec.Body)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 — the verified subject should be the "+
				"principal ACL authorizes (body %s)", rec.Code, rec.Body)
		}
	})

	t.Run("unverified request is denied before ACL", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("SPA shell stays reachable through the real router", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusUnauthorized {
			t.Error("SPA shell was gated (RR-T15E): operators lose the recovery surface")
		}
	})
}

// TestJWTGate_AssertedRolesReachACL is the systemic guard TKT-OJL2GN's 5-whys
// identified as missing: it drives a signed assertion CARRYING ROLES through the
// real NewRouter stack and asserts the mapped role produces an ACL decision.
//
// The prior gate test (TestJWTGate_RouterChainOrder) proved the verified subject
// reaches the ACL but stopped short of the claims — which is exactly the gap
// that let the gate ship stamping a subject-only Principal while the asserted-
// role machinery sat inert. This test pins the whole identity path
// (verify → stamp → resolve → ACL), so a future rework of the gate or the
// resolver cannot silently sever claims again.
func TestJWTGate_AssertedRolesReachACL(t *testing.T) {
	const header = "X-Auth-Assertion"

	// A policy where the ONLY way to read a ticket is an asserted "admin" claim
	// mapped to the viewer role. The principal has no assignments entry and no
	// membership edge, so a 200 can only come from the asserted role flowing
	// through the gate.
	newApp := func(t *testing.T) *App {
		t.Helper()
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
		app.acl = mustNewACL(t, &acl.Policy{
			Roles:         map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
			AssertedRoles: map[string]acl.RoleList{"admin": {"viewer"}},
		}, app.store)
		return app
	}

	get := func(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
		req.Header.Set(header, "good.jwt.token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("asserted role grants read through the real gate", func(t *testing.T) {
		app := newApp(t)
		if err := app.SetJWTGate(JWTGateConfig{
			Verifier: gateVerifier{
				validToken: "good.jwt.token", subject: "usr_nobody",
				orgID: "org_acme", roles: []string{"admin"},
			},
			HeaderName: header,
		}); err != nil {
			t.Fatalf("SetJWTGate: %v", err)
		}

		rec := get(t, app.NewRouter())
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 — the asserted 'admin' claim maps to "+
				"viewer and should grant read; got %s", rec.Code, rec.Body)
		}
	})

	t.Run("without the claim the same request is denied", func(t *testing.T) {
		// The negative half: identical setup, no roles on the assertion. Proves
		// the 200 above came from the claim, not from an accidental open grant.
		app := newApp(t)
		if err := app.SetJWTGate(JWTGateConfig{
			Verifier: gateVerifier{
				validToken: "good.jwt.token", subject: "usr_nobody",
				roles: nil,
			},
			HeaderName: header,
		}); err != nil {
			t.Fatalf("SetJWTGate: %v", err)
		}

		rec := get(t, app.NewRouter())
		if rec.Code == http.StatusOK {
			t.Fatalf("status = 200 for a principal holding no asserted role — the "+
				"gate is granting read without the claim (body %s)", rec.Body)
		}
	})
}
