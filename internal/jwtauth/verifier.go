// Package jwtauth verifies signed identity assertions (ES256 JWTs) from an
// upstream OIDC identity proxy against its published JWKS. It is deliberately
// provider-agnostic: any proxy that injects a signed JWT (oauth2-proxy,
// Pomerium, Keycloak, Pratique, ...) works by pointing it at the right issuer +
// JWKS URL. The package owns only verification + the stable subject extraction
// the caller needs; identity mapping and provisioning live elsewhere.
package jwtauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalid is returned when an assertion was successfully EVALUATED and
// rejected — a bad signature, a wrong iss/aud, an expired or malformed token.
// The specific reason is never surfaced in an HTTP response (the input is
// attacker-controlled), but callers may classify it for server-side logging.
var ErrInvalid = errors.New("jwtauth: assertion failed verification")

// ErrKeysUnavailable reports that verification could not be COMPLETED because
// the JWKS — the root of trust — was unreachable, as distinct from [ErrInvalid],
// which reports an assertion that WAS evaluated and rejected.
//
// Both deny the request; they differ in who must act. ErrInvalid is a client
// fault and is expected in normal operation (a session's token expires).
// ErrKeysUnavailable is an operator fault: rela cannot reach its IdP, and no
// assertion can be verified until that is fixed. Under a fail-closed identity
// policy that is an outage, so it warrants an operational alert rather than
// per-request auth noise.
//
// It deliberately does NOT wrap ErrInvalid: the conditions are genuinely
// different, and a caller that means "deny" should test err != nil.
var ErrKeysUnavailable = errors.New("jwtauth: signing keys unavailable")

// jwksRefreshInterval bounds how long a signing key the IdP has REMOVED (e.g.
// after a compromise) stays accepted: at most one refresh interval. Shorter than
// keyfunc's 1h default because this is an auth root of trust.
const jwksRefreshInterval = 10 * time.Minute

// Config pins the values every assertion must satisfy. All three are mandatory:
// Issuer and Audience are the confused-deputy guards, JWKSURL is the (https) root
// of trust.
type Config struct {
	Issuer   string // expected iss
	Audience string // expected aud — this server's id, per the proxy config
	JWKSURL  string // the proxy's JWKS endpoint (https)
}

// Verifier checks assertions against a cached, auto-refreshing JWKS.
type Verifier struct {
	cfg  Config
	kf   keyfunc.Keyfunc
	opts []jwt.ParserOption
}

// New constructs a Verifier, validating the mandatory pins and fetching the JWKS
// up front. A missing pin, a non-https JWKS URL, or an UNREACHABLE JWKS all fail
// here — so a misconfigured or IdP-down server fails loudly at startup rather
// than booting a verifier that rejects every token. Under the fail-closed
// identity policy that would deny every API request, so catching it at startup
// is the difference between a server that won't boot and one that boots and
// serves nothing.
func New(ctx context.Context, cfg Config) (*Verifier, error) {
	if cfg.Issuer == "" || cfg.Audience == "" || cfg.JWKSURL == "" {
		return nil, errors.New("jwtauth: issuer, audience and jwks url are all required")
	}
	if err := requireHTTPS(cfg.JWKSURL); err != nil {
		return nil, err
	}

	// keyfunc's default HTTP client sets NoErrorReturnFirstHTTPReq=true, which
	// makes an unreachable-JWKS-at-startup return successfully (then reject every
	// token). Override it to false so the up-front fetch genuinely gates startup.
	// Also bound the refresh interval + the unknown-kid rate wait so a hostile or
	// slow IdP can't stall the request path unboundedly.
	failFast := false
	kf, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{cfg.JWKSURL}, keyfunc.Override{
		NoErrorReturnFirstHTTPReq: &failFast,
		RefreshInterval:           jwksRefreshInterval,
		HTTPTimeout:               10 * time.Second,
		RateLimitWaitMax:          5 * time.Second,
		RefreshErrorHandlerFunc:   refreshErrorHandler,
	})
	if err != nil {
		return nil, fmt.Errorf("jwtauth: load jwks %q: %w", cfg.JWKSURL, err)
	}
	// Parser options: ES256 only (reject alg:none / RS-vs-ES confusion), pinned
	// issuer, strict audience, and a required expiry.
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithAudience(cfg.Audience),
		jwt.WithExpirationRequired(),
	}
	return &Verifier{cfg: cfg, kf: kf, opts: opts}, nil
}

// refreshErrorHandler reports a failed background JWKS refresh. keyfunc calls it
// with the JWKS URL and expects the per-URL handler back.
//
// A failed refresh is NOT itself an outage: jwkset replaces the cached key set
// only after a fetch, status check and decode all succeed, so a failure leaves
// the last-known-good keys in place and verification carries on unaffected. It
// becomes an outage only if the IdP rotates its signing key while the JWKS stays
// unreachable — then the new `kid` is absent from the cached set, the
// synchronous refresh also fails, and requests are denied.
//
// The message says so explicitly so an operator can judge urgency from the log
// line alone rather than paging on a transient blip. It fires at most once per
// jwksRefreshInterval, so it needs no extra rate limiting.
func refreshErrorHandler(u string) func(ctx context.Context, err error) {
	return func(ctx context.Context, err error) {
		slog.ErrorContext(ctx, "jwtauth: JWKS background refresh failed; "+
			"verification continues against the cached key set. This becomes an "+
			"outage only if the IdP rotates its signing key before the JWKS is "+
			"reachable again.",
			"jwks_url", u, "error", err)
	}
}

// VerifySubject verifies a request assertion and returns its subject (the stable
// OIDC `sub` — the identity anchor). Any failure, or an empty subject, yields an
// error wrapping either [ErrInvalid] or [ErrKeysUnavailable] — see classify.
// ctx bounds the JWKS refresh a rare unknown-kid may trigger, so a slow IdP can't
// outlast the request's deadline.
func (v *Verifier) VerifySubject(ctx context.Context, raw string) (string, error) {
	claims := jwt.MapClaims{}
	if _, err := jwt.NewParser(v.opts...).ParseWithClaims(raw, claims, v.kf.KeyfuncCtx(ctx)); err != nil {
		return "", fmt.Errorf("%w: %w", classify(err), err)
	}
	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		// A well-formed, correctly-signed token whose subject is missing or
		// empty: definitively a client-side fault, never a key problem.
		return "", ErrInvalid
	}
	return sub, nil
}

// classify decides whether a parse failure means "this assertion is bad"
// ([ErrInvalid]) or "I could not reach my root of trust" ([ErrKeysUnavailable]).
//
// The distinction exists because an unknown `kid` — what a key rotation looks
// like — makes jwkset perform a SYNCHRONOUS JWKS refresh bounded by the request
// context (and by RateLimitWaitMax). When the JWKS is unreachable that surfaces
// as a context or key-retrieval error, not as a signature failure, and it is an
// operator-actionable outage rather than an auth event.
//
// The default is deliberately ErrInvalid: misclassifying an outage as invalid
// costs a missed alert, while misclassifying a bad signature as an outage costs
// a false page. Both still deny, so neither direction is a security hole — bias
// toward the quieter one.
func classify(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		// The unknown-kid refresh outlived the request deadline.
		return ErrKeysUnavailable
	case errors.Is(err, jwkset.ErrKeyNotFound):
		// The kid is absent from the cached set and a refresh did not supply it.
		return ErrKeysUnavailable
	default:
		return ErrInvalid
	}
}

// WebhookClaims is the subset of a verified webhook JWT the receiver acts on.
// It is a typed projection — the package never leaks jwt.MapClaims across its
// boundary — so a caller can't accidentally trust an unverified claim.
type WebhookClaims struct {
	Event  string // the "event" claim, e.g. "membership.created"
	UserID string // the "user_id" claim — the subject the event concerns
	OrgID  string // the "org_id" claim — the tenant the event concerns
	ID     string // the "jti" claim, if present — used for replay dedup
}

// WebhookVerifier verifies signed webhook JWTs. It is DISTINCT from the identity
// Verifier on purpose: a webhook is a server-to-server notification, not a
// request assertion, so it carries its OWN audience (the value the IdP stamps on
// callbacks, e.g. "rela-webhook"). Pinning that audience separately — rather than
// relaxing the audience check — keeps the confused-deputy guard intact: an
// identity assertion (aud = this server) can't be replayed as a webhook and vice
// versa. Same JWKS + issuer + ES256 + required-expiry as the identity path.
type WebhookVerifier struct {
	kf   keyfunc.Keyfunc
	opts []jwt.ParserOption
}

// NewWebhookVerifier builds a webhook verifier sharing an existing identity
// Verifier's JWKS + issuer, but pinning webhookAudience instead of the identity
// audience. Reusing the identity Verifier's already-fetched, auto-refreshing JWKS
// means no second network fetch and one refresh cadence. webhookAudience is
// required — an empty one would accept any audience, reopening the confused-
// deputy hole this type exists to close.
func NewWebhookVerifier(idv *Verifier, webhookAudience string) (*WebhookVerifier, error) {
	if idv == nil {
		return nil, errors.New("jwtauth: identity verifier is required")
	}
	if webhookAudience == "" {
		return nil, errors.New("jwtauth: webhook audience is required")
	}
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithIssuer(idv.cfg.Issuer),
		jwt.WithAudience(webhookAudience),
		jwt.WithExpirationRequired(),
	}
	return &WebhookVerifier{kf: idv.kf, opts: opts}, nil
}

// VerifyWebhook verifies a webhook JWT and projects the claims the receiver acts
// on. Any signature/issuer/audience/expiry failure yields ErrInvalid. The event,
// user_id, and org_id claims are read as strings; a missing one comes back "" and
// the caller decides whether that's fatal (a membership event with no user_id is
// unusable, for instance). ctx bounds any JWKS refresh a rare unknown-kid needs.
func (v *WebhookVerifier) VerifyWebhook(ctx context.Context, raw string) (WebhookClaims, error) {
	claims := jwt.MapClaims{}
	if _, err := jwt.NewParser(v.opts...).ParseWithClaims(raw, claims, v.kf.KeyfuncCtx(ctx)); err != nil {
		return WebhookClaims{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return WebhookClaims{
		Event:  stringClaim(claims, "event"),
		UserID: stringClaim(claims, "user_id"),
		OrgID:  stringClaim(claims, "org_id"),
		ID:     stringClaim(claims, "jti"),
	}, nil
}

// stringClaim reads a string claim, returning "" when absent or not a string.
// Non-string is treated as absent rather than an error: a malformed claim is
// indistinguishable from a missing one for the receiver's purposes (it acts on
// the value only when non-empty).
func stringClaim(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	return ""
}

// requireHTTPS rejects a JWKS URL that isn't https — the JWKS is the root of
// trust for every signature, so fetching it over cleartext would let an on-path
// attacker substitute their own signing key (a full auth bypass). A loopback
// host is exempted so local test servers (httptest, http://127.0.0.1) work.
func requireHTTPS(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("jwtauth: invalid jwks url %q: %w", raw, err)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("jwtauth: jwks url must be https (got %q)", raw)
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "127.")
}
