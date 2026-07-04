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
