package dataentry

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Reserved-principal rejection at the API boundary (TKT-9PCL7D).
//
// `system:scheduler` and `system:provisioner` are grantable in acl.yaml — the
// DEC-O59WM4 migration binds the former to a role with `read: ["*"]` — and
// internal/acl resolves them by a plain map lookup on the raw string, with no
// notion of where the principal came from. So a reserved name arriving over
// HTTP inherits the scheduler's grants and the ACL cannot tell it apart from
// the real scheduler.
//
// These tests pin the rejection on every request-path source: the proxy header
// and $RELA_DATAENTRY_USER (via stampAuditPrincipal) and the verified assertion
// (via verifiedPrincipal, reached through the production JWT gate).

// reservedUsers spans the two live constants, an identity that does not exist
// yet, and the bare prefix — the guard is a namespace rule, not a two-name
// blocklist, so a future system:* identity must be covered before anyone
// remembers to update a list.
var reservedUsers = []string{
	principal.UserScheduler,
	principal.UserProvisioner,
	"system:future",
	"system:",
}

// capturePrincipal builds a handler that records the principal it was reached
// with, plus a flag proving it ran at all. On a rejection the flag must stay
// false: a 403 whose handler still executed would have already done the work.
func capturePrincipal(reached *bool, got *principal.Principal) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		*got = principal.From(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

// TestStampAuditPrincipal_RejectsReservedHeaderUser is AC1: a reserved name in
// the trusted-proxy header is refused on the API surface.
func TestStampAuditPrincipal_RejectsReservedHeaderUser(t *testing.T) {
	const header = "X-Remote-User"
	for _, user := range reservedUsers {
		t.Run(user, func(t *testing.T) {
			var reached bool
			var got principal.Principal
			h := stampAuditPrincipal(capturePrincipal(&reached, &got),
				ChainResolvers(HeaderPrincipalResolver(header)))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
			req.Header.Set(header, user)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
			if reached {
				t.Errorf("handler ran with principal %q; the request must be "+
					"refused before it reaches a handler", got.User)
			}
		})
	}
}

// TestStampAuditPrincipal_RejectsReservedEnvUser is AC2. The env resolver sits
// FIRST in the production chain (cmd/rela-server), so it outranks the header
// and is the highest-priority spoofing surface of the two.
func TestStampAuditPrincipal_RejectsReservedEnvUser(t *testing.T) {
	t.Setenv(envDataEntryUser, principal.UserProvisioner)

	var reached bool
	var got principal.Principal
	h := stampAuditPrincipal(capturePrincipal(&reached, &got),
		ChainResolvers(EnvPrincipalResolver(), HeaderPrincipalResolver("X-Remote-User")))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if reached {
		t.Errorf("handler ran with principal %q, want the request refused", got.User)
	}
}

// TestStampAuditPrincipal_ReservedRejectionIsNotADowngrade is the sharp edge of
// AC1. The tempting implementation drops a reserved name and lets the chain
// fall through to `unknown`, which turns an impersonation attempt into an
// ordinary-looking anonymous request — no error, no signal, and the write still
// happens. Assert the loud failure explicitly.
func TestStampAuditPrincipal_ReservedRejectionIsNotADowngrade(t *testing.T) {
	const header = "X-Remote-User"
	var reached bool
	var got principal.Principal
	h := stampAuditPrincipal(capturePrincipal(&reached, &got),
		ChainResolvers(HeaderPrincipalResolver(header)))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set(header, principal.UserScheduler)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if reached {
		t.Fatalf("handler ran as %q — a reserved principal was silently "+
			"downgraded instead of rejected", got.User)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	// The body names the rule. Per CLAUDE.md the configuration is not a secret,
	// so telling the operator WHY is correct and speeds up diagnosing the
	// misconfigured proxy that is the likeliest cause.
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode problem+json: %v", err)
	}
	if typ, _ := body["type"].(string); !strings.HasSuffix(typ, "reserved_principal") {
		t.Errorf("error type = %v, want it to name reserved_principal", body["type"])
	}
}

// TestStampAuditPrincipal_ReservedOutsideAPIDegradesNotErrors guards the
// RR-T15E constraint. stampAuditPrincipal runs on EVERY request, so 403-ing
// unconditionally would render the SPA shell and static assets as raw JSON the
// moment a proxy is misconfigured — locking operators out of the surface they
// need to fix it. Outside /api/ the identity is stripped to the anonymous
// default instead; nothing there reads the principal for authorization.
func TestStampAuditPrincipal_ReservedOutsideAPIDegradesNotErrors(t *testing.T) {
	const header = "X-Remote-User"
	for _, path := range []string{"/", "/static/app.js", "/index.html"} {
		t.Run(path, func(t *testing.T) {
			var reached bool
			var got principal.Principal
			h := stampAuditPrincipal(capturePrincipal(&reached, &got),
				ChainResolvers(HeaderPrincipalResolver(header)))

			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			req.Header.Set(header, principal.UserScheduler)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d — the SPA must stay reachable",
					rec.Code, http.StatusOK)
			}
			if !reached {
				t.Fatal("handler did not run; the SPA shell must still be served")
			}
			if principal.IsReserved(got.User) {
				t.Errorf("principal = %q, want the reserved identity stripped", got.User)
			}
			if got.User != "unknown" {
				t.Errorf("principal = %q, want the anonymous default", got.User)
			}
		})
	}
}

// TestStampAuditPrincipal_OrdinaryUsersUnaffected is the regression guard on
// the common path: near-miss names that merely resemble the reserved namespace
// must keep working.
func TestStampAuditPrincipal_OrdinaryUsersUnaffected(t *testing.T) {
	const header = "X-Remote-User"
	// "System:Scheduler" is deliberately allowed: the ACL matches assignment
	// keys exactly, so it confers no scheduler grant. Case-folding the guard
	// would cost this user their login for no security gain.
	for _, user := range []string{
		"alice", "alice@example.com", "systemscheduler",
		"my-system:scheduler", "webhook:user-created", "System:Scheduler",
	} {
		t.Run(user, func(t *testing.T) {
			var reached bool
			var got principal.Principal
			h := stampAuditPrincipal(capturePrincipal(&reached, &got),
				ChainResolvers(HeaderPrincipalResolver(header)))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
			req.Header.Set(header, user)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d — ordinary user was rejected",
					rec.Code, http.StatusOK)
			}
			if !reached || got.User != user {
				t.Errorf("principal = %q, want %q", got.User, user)
			}
		})
	}
}

// TestRequireVerifiedJWT_RejectsReservedSubject is AC3, the test that exists
// because of the bypass this ticket nearly shipped with. The gate re-stamps the
// principal AFTER stampAuditPrincipal, so a guard placed only in the resolver
// chain is silently defeated whenever the gate is installed — which is the
// production configuration. A valid SIGNATURE proves the IdP issued the subject;
// it does not prove the subject is not a reserved internal name.
func TestRequireVerifiedJWT_RejectsReservedSubject(t *testing.T) {
	const header = "X-Auth-Assertion"
	for _, user := range reservedUsers {
		t.Run(user, func(t *testing.T) {
			cfg := JWTGateConfig{
				Verifier:   gateVerifier{validToken: "good.jwt.token", subject: user},
				HeaderName: header,
			}
			h := newGateHandler(t, cfg)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
			req.Header.Set(header, "good.jwt.token")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				t.Fatalf("a validly signed assertion for %q was accepted; the "+
					"gate's re-stamp bypassed the reserved-principal guard", user)
			}
			if body := rec.Body.String(); strings.Contains(body, user) {
				t.Errorf("denial body echoed the subject %q: %s", user, body)
			}
		})
	}
}

// TestRequireVerifiedJWT_ReservedSubjectLogsSecurityWarn pins the log
// CLASSIFICATION end-to-end, not merely the denial (RR-OJRCNY).
//
// A reserved subject wearing a leading control character is the case that broke:
// sanitizeUser strips it, so the subject IS reserved by the time the decision is
// made, but a classifier re-deriving from the raw `id.Subject` would see an
// ordinary string and emit the benign "unusable after sanitization" INFO. The
// request is denied either way — what is lost is the one WARN the docs tell
// operators to grep for, on precisely the variant an attacker would send
// because it reads as IdP noise.
func TestRequireVerifiedJWT_ReservedSubjectLogsSecurityWarn(t *testing.T) {
	const header = "X-Auth-Assertion"
	for _, tc := range []struct{ name, subject string }{
		{"plain", principal.UserScheduler},
		{"leading control char", "\x01" + principal.UserScheduler},
		{"leading DEL", "\x7f" + principal.UserScheduler},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf,
				&slog.HandlerOptions{Level: slog.LevelDebug})))
			t.Cleanup(func() { slog.SetDefault(prev) })

			cfg := JWTGateConfig{
				Verifier:   gateVerifier{validToken: "good.jwt.token", subject: tc.subject},
				HeaderName: header,
			}
			h := newGateHandler(t, cfg)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
			req.Header.Set(header, "good.jwt.token")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				t.Fatalf("subject %q was accepted", tc.subject)
			}
			logged := buf.String()
			if !strings.Contains(logged, "rejected reserved principal") {
				t.Errorf("no security WARN for %q; got: %s", tc.subject, logged)
			}
			if strings.Contains(logged, "unusable after sanitization") {
				t.Errorf("reserved subject %q misreported as merely unusable: %s",
					tc.subject, logged)
			}
		})
	}
}

// TestRequireVerifiedJWT_UnusableSubjectStillLogsInfo is the other direction:
// the classification must discriminate, or promoting everything to WARN would
// make the security signal meaningless.
func TestRequireVerifiedJWT_UnusableSubjectStillLogsInfo(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	const header = "X-Auth-Assertion"
	cfg := JWTGateConfig{
		// Control characters only: sanitizes away to "" — an IdP/claims problem,
		// not an impersonation attempt.
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: "\x01\x02"},
		HeaderName: header,
	}
	h := newGateHandler(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set(header, "good.jwt.token")
	h.ServeHTTP(httptest.NewRecorder(), req)

	logged := buf.String()
	if !strings.Contains(logged, "unusable after sanitization") {
		t.Errorf("expected the unusable-subject INFO; got: %s", logged)
	}
	if strings.Contains(logged, "rejected reserved principal") {
		t.Errorf("unusable subject misreported as reserved: %s", logged)
	}
}

// TestStampAuditPrincipal_RejectsControlCharPrefixedReserved pins the header
// path for the same class: sanitizeUser maps the control character to a space
// which is then trimmed, so the value IS reserved by the time it is checked.
func TestStampAuditPrincipal_RejectsControlCharPrefixedReserved(t *testing.T) {
	const header = "X-Remote-User"
	var reached bool
	var got principal.Principal
	h := stampAuditPrincipal(capturePrincipal(&reached, &got),
		ChainResolvers(HeaderPrincipalResolver(header)))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	// Set directly: http.Header.Set would reject a control character outright,
	// but a non-Go client can put one on the wire.
	req.Header[header] = []string{"\x01" + principal.UserScheduler}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if reached {
		t.Errorf("handler ran as %q", got.User)
	}
}

// TestRequireVerifiedJWT_ReservedSubjectRejectedOnMCPPath is AC5: the remote
// MCP endpoint shares verifiedPrincipal via toolForPath, so it must reject too.
// Worth its own test because MCP is the surface an agent reaches, where a
// blanket-read identity would be most valuable to an attacker.
func TestRequireVerifiedJWT_ReservedSubjectRejectedOnMCPPath(t *testing.T) {
	const header = "X-Auth-Assertion"
	cfg := JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: principal.UserScheduler},
		HeaderName: header,
	}
	h := newGateHandler(t, cfg)

	req := httptest.NewRequest(http.MethodPost, MCPPath, http.NoBody)
	req.Header.Set(header, "good.jwt.token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("reserved subject accepted on the MCP endpoint %q", MCPPath)
	}
}

// TestRequireVerifiedJWT_OrdinarySubjectStillWorks pins that the new check in
// verifiedPrincipal did not break the normal verified path.
func TestRequireVerifiedJWT_OrdinarySubjectStillWorks(t *testing.T) {
	const header = "X-Auth-Assertion"
	cfg := JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: "usr_abc123"},
		HeaderName: header,
	}
	h := newGateHandler(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set(header, "good.jwt.token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "usr_abc123" {
		t.Errorf("principal = %q, want the verified subject", got)
	}
}

// TestVerifiedPrincipal_RejectsReserved covers the projection directly, so the
// rule is pinned at the function the deprecated resolver and the gate share
// rather than only through one of its callers.
func TestVerifiedPrincipal_RejectsReserved(t *testing.T) {
	for _, user := range reservedUsers {
		t.Run(user, func(t *testing.T) {
			_, reason := verifiedPrincipal(AssertedIdentity{Subject: user},
				principal.ToolDataEntry)
			if reason.ok() {
				t.Errorf("verifiedPrincipal accepted reserved subject %q", user)
			}
			// The REASON matters, not just the refusal: the gate picks its log
			// line from it, so a reserved subject reported as merely unusable
			// would deny correctly while hiding the security signal (RR-OJRCNY).
			if reason != rejectReserved {
				t.Errorf("reason = %v, want rejectReserved for %q", reason, user)
			}
		})
	}

	// Noise that sanitizeUser would strip must not smuggle a reserved name past
	// the classification. These are the cases where IsReserved on the RAW value
	// disagrees with IsReserved on the SANITIZED value; both must land on
	// rejectReserved (RR-NQK412, RR-O4VZW0).
	for _, tc := range []struct{ name, subject string }{
		{"whitespace padded", "  system:scheduler  "},
		{"leading C0", "\x01system:scheduler"},
		{"leading DEL", "\x7fsystem:scheduler"},
		{"leading tab", "\tsystem:scheduler"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, reason := verifiedPrincipal(
				AssertedIdentity{Subject: tc.subject}, principal.ToolDataEntry)
			if reason != rejectReserved {
				t.Errorf("verifiedPrincipal(%q) reason = %v, want rejectReserved",
					tc.subject, reason)
			}
		})
	}

	// An genuinely unusable subject must NOT be misreported as reserved — the
	// classification has to discriminate in both directions or the WARN becomes
	// noise.
	if _, reason := verifiedPrincipal(AssertedIdentity{Subject: "\x01\x02"},
		principal.ToolDataEntry); reason != rejectUnusable {
		t.Errorf("control-only subject reason = %v, want rejectUnusable", reason)
	}
}

// TestReservedPrincipalRejection_IsLogged is AC8. An operator whose proxy has
// started stamping a reserved name needs to find out from the logs; a silent
// 403 leaves them guessing. The record must carry the attempted name and the
// remote address — both attacker-supplied or already-known, no secret material.
func TestReservedPrincipalRejection_IsLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	const header = "X-Remote-User"
	h := stampAuditPrincipal(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), ChainResolvers(HeaderPrincipalResolver(header)))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody)
	req.Header.Set(header, principal.UserScheduler)
	req.RemoteAddr = "203.0.113.7:54321"
	h.ServeHTTP(httptest.NewRecorder(), req)

	logged := buf.String()
	for _, want := range []string{principal.UserScheduler, "203.0.113.7", "/api/v1/entities"} {
		if !strings.Contains(logged, want) {
			t.Errorf("log does not mention %q; got: %s", want, logged)
		}
	}
}
