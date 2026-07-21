package dataentry

import (
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
	// Verifier checks the assertion. Required.
	Verifier subjectVerifier
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

// requireVerifiedJWT enforces that every data-API request carries a
// cryptographically verified identity assertion, denying with 401 when it does
// not. It is the fail-closed counterpart to [JWTPrincipalResolver]: where the
// resolver returns a zero Principal on failure and lets the chain fall through
// to the next source, this gate refuses the request outright.
//
// **Why a gate and not a resolver.** When JWT identity is enabled it is the ONLY
// identity source — the wiring rejects --principal-header and $RELA_DATAENTRY_USER
// alongside it at startup. A fall-through chain would mean a JWKS disruption
// silently downgrades a verified identity to a spoofable proxy-trusted header, an
// attacker-triggerable auth bypass. With exactly one source there is nothing to
// fall through TO, so the resolver's zero-Principal signaling has no job to do
// and [PrincipalResolver] keeps its single-return signature.
//
// **Scope.** Only [isAPIPath] requests are gated, extending the RR-T15E invariant
// that [attachACLRequest] already relies on: the SPA shell and static assets must
// stay reachable so a client can load and render a signed-out state, and so a
// misconfiguration does not lock operators out of the recovery surface. Those
// routes serve no entity data; every API call the SPA makes is still gated. The
// self-authenticating IdP webhook (POST /webhooks/idp) verifies a signed body
// with its own audience and is deliberately outside this gate — it will never
// carry an identity assertion, and gating it would reject every legitimate
// callback.
//
// **Denials never explain themselves.** The 401 body carries no verification
// detail: the token is attacker-controlled input, and echoing why it failed is an
// oracle. The reason is logged server-side instead (RR-372L).
func requireVerifiedJWT(next http.Handler, cfg JWTGateConfig) http.Handler {
	g := &jwtGate{cfg: cfg}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		raw := stripBearer(r.Header.Get(cfg.HeaderName))
		if raw == "" {
			// Debug, not Info: unauthenticated probes and scanners are routine
			// and would flood a production log at any higher level.
			slog.DebugContext(r.Context(), "jwt gate: no assertion presented",
				"path", r.URL.Path, "method", r.Method, "remote_addr", r.RemoteAddr)
			g.deny(w, r)
			return
		}

		sub, err := cfg.Verifier.VerifySubject(r.Context(), raw)
		if err != nil {
			g.logFailure(r, err)
			g.deny(w, r)
			return
		}

		// Same input filtering as JWTPrincipalResolver: the subject is an opaque
		// IdP-controlled id, but cap length and strip control characters so a
		// hostile IdP cannot corrupt the audit JSONL stream. A control-only
		// subject sanitizes to "" and is denied rather than silently downgraded.
		user := sanitizeUser(sub)
		if user == "" {
			slog.InfoContext(r.Context(), "jwt gate: verified subject is unusable after sanitization",
				"path", r.URL.Path, "method", r.Method, "remote_addr", r.RemoteAddr)
			g.deny(w, r)
			return
		}

		g.noteRecovery(r)
		ctx := principal.With(r.Context(), principal.Principal{
			User: user,
			Tool: principal.ToolDataEntry,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
	// RFC 6750: advertise the scheme when the assertion rides the standard
	// Authorization header, so a client knows how to authenticate. For a custom
	// proxy-injected header the challenge would be meaningless.
	if strings.EqualFold(g.cfg.HeaderName, "Authorization") {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	writeV1Error(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required", "")
}
