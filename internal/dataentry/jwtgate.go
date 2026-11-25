package dataentry

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// denialLogInterval bounds how often the gate logs a denial, per failure class.
// Under the failure this guards — the IdP rotating its signing key while the
// JWKS is unreachable — EVERY request is denied, so an unsampled line would
// flood the log exactly when it is least affordable and least informative. The
// first occurrence always logs; subsequent ones are counted and folded into the
// next line, or reported by [jwtGate.noteRecovery] when the burst ends.
const denialLogInterval = 10 * time.Second

// JWTGateConfig configures fail-closed verified-JWT identity. Install it with
// [App.SetJWTGate]; when set, [App.NewRouter] wraps the API surface in
// [requireVerifiedJWT] and the principal-resolver chain is bypassed entirely for
// those requests.
type JWTGateConfig struct {
	// Verifier checks the assertion and projects its claims. Required.
	//
	// It returns the full [AssertedIdentity] (subject + org + roles), not just
	// the subject, so the gate can stamp asserted roles onto the Principal it
	// installs. A subject-only verifier here would authenticate correctly but
	// silently strip every asserted role before the ACL sees it (TKT-OJL2GN).
	Verifier assertionVerifier
	// HeaderName is the request header carrying the assertion. Required.
	HeaderName string
	// KeysUnavailable reports whether a verification error means the JWKS was
	// unreachable (an operator-actionable outage) rather than the assertion
	// being bad (a client fault). Both deny; they differ only in how they are
	// logged.
	//
	// It is injected as a predicate rather than imported so this package stays
	// independent of internal/jwtauth — the wiring layer supplies
	// errors.Is(err, jwtauth.ErrKeysUnavailable). Optional: a nil predicate
	// classifies every failure as a client fault.
	KeysUnavailable func(error) bool
}

// jwtGate holds the config plus the per-class log samplers. One instance per
// router; the samplers are independent so a flood of one class cannot mask the
// other.
type jwtGate struct {
	cfg JWTGateConfig

	invalid         logSampler
	keysUnavailable logSampler
}

// logSampler rate-limits one class of repeated log line, reporting how many
// occurrences were folded into the line it does emit.
type logSampler struct {
	mu         sync.Mutex
	lastLogged time.Time
	suppressed int
}

// sample reports whether this occurrence should be logged, along with how many
// were suppressed since the last logged one.
//
// Note the residual: when a burst ends, whatever was suppressed after the last
// emitted line is never reported — there is no timer to flush it. That is an
// accepted limitation rather than a bug; a trailing goroutine per sampler would
// cost more than the count is worth. The count is not lost silently, though:
// [jwtGate.noteRecovery] flushes it on the next SUCCESSFUL verification, which
// is exactly when an operator wants to know how long the outage ran.
func (s *logSampler) sample() (suppressed int, shouldLog bool) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.lastLogged.IsZero() && now.Sub(s.lastLogged) < denialLogInterval {
		s.suppressed++
		return 0, false
	}
	suppressed = s.suppressed
	s.suppressed = 0
	s.lastLogged = now
	return suppressed, true
}

// drain returns and clears any suppressed count without emitting a line.
func (s *logSampler) drain() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := s.suppressed
	s.suppressed = 0
	s.lastLogged = time.Time{}
	return n
}

// requireVerifiedJWT denies with 401 any [isAPIPath] request that lacks a
// verified identity assertion. It is a gate, not a resolver, because JWT is
// then the ONLY identity source: a fall-through would let a JWKS disruption
// downgrade to a spoofable header. The SPA shell, static assets and the
// self-authenticating IdP webhook stay outside the gate. A reserved or
// unusable subject is denied, never downgraded. The 401 body never explains
// the failure, which would be an oracle; the reason is logged instead.
func requireVerifiedJWT(next http.Handler, cfg JWTGateConfig) http.Handler {
	g := &jwtGate{cfg: cfg}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		raw := stripBearer(r.Header.Get(cfg.HeaderName))
		if raw == "" {
			slog.DebugContext(r.Context(), "jwt gate: no assertion presented",
				"path", r.URL.Path, "method", r.Method, "remote_addr", r.RemoteAddr)
			g.deny(w, r)
			return
		}

		id, err := cfg.Verifier.VerifyAssertion(r.Context(), raw)
		if err != nil {
			g.logFailure(r, err)
			g.deny(w, r)
			return
		}

		p, reason := verifiedPrincipal(id, toolForPath(r.URL.Path))
		if !reason.ok() {
			if reason == rejectReserved {
				slog.WarnContext(r.Context(), "jwt gate: rejected reserved principal in verified assertion",
					"user", id.Subject,
					"path", r.URL.Path, "method", r.Method, "remote_addr", r.RemoteAddr)
			} else {
				slog.InfoContext(r.Context(), "jwt gate: verified subject is unusable after sanitization",
					"path", r.URL.Path, "method", r.Method, "remote_addr", r.RemoteAddr)
			}
			g.deny(w, r)
			return
		}

		g.noteRecovery(r)
		ctx := withCredentialExpiry(principal.With(r.Context(), p), id.ExpiresAt)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// credentialExpiryKey is the context key for the moment the request's
// verified credential stops being valid. It is unexported, so only the gate
// can set it and a request cannot extend its own stream.
type credentialExpiryKey struct{}

// withCredentialExpiry records when the request's credential expires. It is
// kept apart from [principal.Principal] on purpose: a refreshed token with a
// later expiry is the same identity, and the principal's equality, share key
// and JSON form must not see the difference.
func withCredentialExpiry(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, credentialExpiryKey{}, t)
}

// credentialExpiry returns the expiry the gate recorded. It returns false
// when the request has none (any auth mode other than the JWT gate) and when
// the recorded expiry is zero, which means no expiry is known.
func credentialExpiry(ctx context.Context) (time.Time, bool) {
	t, ok := ctx.Value(credentialExpiryKey{}).(time.Time)
	if !ok || t.IsZero() {
		return time.Time{}, false
	}
	return t, true
}

// logFailure records a verification failure at a level matching who must act:
// Error when the JWKS was unreachable, Info otherwise. An expired token
// mid-session is normal and must not read as an incident; an unreachable root of
// trust must.
//
// BOTH branches are sampled, not just the Error one. A rotation-during-outage
// denies every request, and whether those denials land in the Error or the Info
// branch depends on classify() guessing right — which it deliberately does not
// always do, since it defaults to ErrInvalid. Sampling only the Error branch
// would leave the flood to whichever branch the default happened to pick.
func (g *jwtGate) logFailure(r *http.Request, err error) {
	keysUnavailable := g.cfg.KeysUnavailable != nil && g.cfg.KeysUnavailable(err)

	counter := &g.invalid
	if keysUnavailable {
		counter = &g.keysUnavailable
	}
	suppressed, ok := counter.sample()
	if !ok {
		return
	}

	if !keysUnavailable {
		slog.InfoContext(r.Context(), "jwt gate: assertion failed verification",
			"path", r.URL.Path, "method", r.Method, "remote_addr", r.RemoteAddr,
			"error", err, "suppressed_count", suppressed)
		return
	}
	slog.ErrorContext(r.Context(), "jwt gate: cannot verify assertions — JWKS unavailable. "+
		"All API requests are being denied until the IdP is reachable.",
		"path", r.URL.Path, "method", r.Method, "remote_addr", r.RemoteAddr,
		"error", err, "suppressed_count", suppressed)
}

// noteRecovery reports the end of a denial burst on the first successful
// verification after one. This is what makes the sampler's trailing residual
// observable: without it, the denials suppressed after the last emitted line
// would never be accounted for, and an operator reading the log would see an
// outage begin but never see its size or its end.
func (g *jwtGate) noteRecovery(r *http.Request) {
	keysUnavailable := g.keysUnavailable.drain()
	invalid := g.invalid.drain()
	if keysUnavailable == 0 && invalid == 0 {
		return
	}
	slog.InfoContext(r.Context(), "jwt gate: verification succeeded after a run of denials",
		"suppressed_keys_unavailable", keysUnavailable, "suppressed_invalid", invalid)
}

// deny writes the 401. The detail is deliberately empty — see the type comment.
func (g *jwtGate) deny(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(g.cfg.HeaderName, "Authorization") {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	writeV1Error(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required", "")
}
