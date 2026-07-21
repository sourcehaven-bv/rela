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

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalid is returned when an assertion fails any verification step. The
// specific reason is intentionally not surfaced to request-path callers.
var ErrInvalid = errors.New("jwtauth: assertion failed verification")

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
// than booting a verifier that silently rejects every token (which would degrade
// every request to the chain's fall-through identity).
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

// VerifySubject verifies a request assertion and returns its subject (the stable
// OIDC `sub` — the identity anchor). Any failure, or an empty subject, yields
// ErrInvalid. ctx bounds the JWKS refresh a rare unknown-kid may trigger, so a
// slow IdP can't outlast the request's deadline.
func (v *Verifier) VerifySubject(ctx context.Context, raw string) (string, error) {
	claims := jwt.MapClaims{}
	if _, err := jwt.NewParser(v.opts...).ParseWithClaims(raw, claims, v.kf.KeyfuncCtx(ctx)); err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return "", ErrInvalid
	}
	return sub, nil
}

// AssertionClaims is the subset of a verified identity assertion the principal
// resolver acts on. Like [WebhookClaims] it is a typed projection — the package
// never leaks jwt.MapClaims across its boundary — so a caller can't accidentally
// trust an unverified claim.
//
// Subject is the only field callers should treat as required; the rest come back
// zero when absent. An assertion from an OIDC proxy that doesn't model orgs or
// roles is still perfectly valid, so a missing claim is not an error.
type AssertionClaims struct {
	Subject string   // the "sub" claim — the stable identity anchor
	Email   string   // the "email" claim, if present
	OrgID   string   // the "org_id" claim — the tenant the session is scoped to
	OrgSlug string   // the "org_slug" claim — human-readable tenant name
	Roles   []string // the "roles" claim — bare role names, scoped to OrgID
}

// Claim-projection bounds. A verified signature proves the IdP asserted these
// values, NOT that they are small: a buggy or compromised IdP, or a user with
// pathological group membership, can mint an assertion with thousands of roles.
// Every downstream consumer would then pay per-role costs on every request
// (attribution slices, audit-log lines). Bounding here rather than at a call
// site means each future consumer inherits the limit instead of re-deriving it.
const (
	maxRoles     = 32
	maxRoleRunes = 256
)

// VerifyAssertion verifies a request assertion and projects the identity claims
// the principal resolver needs. Any signature/issuer/audience/expiry failure, or
// an empty subject, yields ErrInvalid — identical to [VerifySubject], which this
// widens rather than replaces.
//
// Absent org/role claims are NOT an error (see [AssertionClaims]). Roles are
// bounded per the constants above; a non-string element is skipped rather than
// failing the request, matching stringClaim's treatment of a malformed claim as
// an absent one.
func (v *Verifier) VerifyAssertion(ctx context.Context, raw string) (AssertionClaims, error) {
	claims := jwt.MapClaims{}
	if _, err := jwt.NewParser(v.opts...).ParseWithClaims(raw, claims, v.kf.KeyfuncCtx(ctx)); err != nil {
		return AssertionClaims{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return AssertionClaims{}, ErrInvalid
	}
	return AssertionClaims{
		Subject: sub,
		Email:   stringClaim(claims, "email"),
		OrgID:   stringClaim(claims, "org_id"),
		OrgSlug: stringClaim(claims, "org_slug"),
		Roles:   stringSliceClaim(claims, "roles"),
	}, nil
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

// stringSliceClaim reads a string-array claim, returning nil when absent, null,
// or not an array. Non-string elements are skipped for the same reason
// stringClaim treats a non-string as absent: a malformed element is
// indistinguishable from one the IdP never sent, and dropping it is strictly
// safer than failing a request that is otherwise correctly signed.
//
// The result is bounded by maxRoles/maxRoleRunes; excess is dropped with a
// single warn rather than silently, so an operator can see truncation happened
// without the log itself becoming the amplification.
func stringSliceClaim(claims jwt.MapClaims, key string) []string {
	raw, ok := claims[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, min(len(raw), maxRoles))
	truncated := false
	for _, elem := range raw {
		s, ok := elem.(string)
		if !ok {
			continue
		}
		if len(out) >= maxRoles {
			truncated = true
			break
		}
		out = append(out, truncateRunes(s, maxRoleRunes))
	}
	if truncated {
		slog.Warn("jwtauth: claim truncated",
			"claim", key, "limit", maxRoles, "received", len(raw))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// truncateRunes caps s at n runes, never splitting a multi-byte character.
func truncateRunes(s string, n int) string {
	if len(s) <= n { // len is bytes; runes <= bytes, so this is a safe fast path
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
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
