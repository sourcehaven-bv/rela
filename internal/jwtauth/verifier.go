// Package jwtauth verifies signed identity assertions (ES256 JWTs) from an
// upstream OIDC identity proxy against its published JWKS. It is deliberately
// provider-agnostic: any proxy that injects a signed JWT (oauth2-proxy,
// Pomerium, Keycloak, Pratique, ...) works by pointing it at the right issuer +
// JWKS URL. The package owns only verification + the standard-claim extraction
// the caller needs; identity mapping and provisioning live elsewhere.
package jwtauth

import (
	"context"
	"errors"
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalid is returned when an assertion fails any verification step. The
// specific reason is intentionally not surfaced to request-path callers.
var ErrInvalid = errors.New("jwtauth: assertion failed verification")

// Config pins the values every assertion must satisfy. Issuer and JWKSURL are
// mandatory. Audience is mandatory for request-path assertions (the strict-aud
// confused-deputy guard); it may be empty when verifying tokens that carry no
// audience (see VerifyClaims with requireAudience=false).
type Config struct {
	Issuer   string // expected iss
	Audience string // expected aud for assertions (strict); may be "" for aud-less tokens
	JWKSURL  string // the proxy's JWKS endpoint (https)
}

// Verifier checks assertions against a cached, auto-refreshing JWKS.
type Verifier struct {
	cfg  Config
	kf   keyfunc.Keyfunc
	opts []jwt.ParserOption
}

// New constructs a Verifier, validating the mandatory pins so a misconfigured
// server fails loudly at startup rather than silently accepting nothing (or,
// worse, everything). It fetches the JWKS once up front.
func New(ctx context.Context, cfg Config) (*Verifier, error) {
	if cfg.Issuer == "" || cfg.JWKSURL == "" {
		return nil, errors.New("jwtauth: issuer and jwks url are required")
	}
	// keyfunc's default client auto-refreshes the JWKS in the background, so a
	// rotated signing key is picked up without a restart.
	kf, err := keyfunc.NewDefaultCtx(ctx, []string{cfg.JWKSURL})
	if err != nil {
		return nil, fmt.Errorf("jwtauth: load jwks %q: %w", cfg.JWKSURL, err)
	}
	// Common parser options: ES256 only (reject alg:none / RS-vs-ES confusion),
	// pinned issuer, and a required expiry.
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithExpirationRequired(),
	}
	return &Verifier{cfg: cfg, kf: kf, opts: opts}, nil
}

// VerifySubject verifies a request-path assertion (strict audience) and returns
// its subject (the stable OIDC `sub` — the identity anchor). Any failure, or an
// empty subject, yields ErrInvalid.
func (v *Verifier) VerifySubject(_ context.Context, raw string) (string, error) {
	claims, err := v.parse(raw, true)
	if err != nil {
		return "", err
	}
	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return "", ErrInvalid
	}
	return sub, nil
}

// VerifyClaims verifies a token and returns its claims map. requireAudience
// toggles the strict-aud check: true for request assertions, false for tokens
// that legitimately carry no audience (e.g. some webhook signatures). Signature,
// issuer, algorithm, and expiry are always enforced.
func (v *Verifier) VerifyClaims(_ context.Context, raw string, requireAudience bool) (jwt.MapClaims, error) {
	return v.parse(raw, requireAudience)
}

func (v *Verifier) parse(raw string, requireAudience bool) (jwt.MapClaims, error) {
	opts := v.opts
	if requireAudience {
		if v.cfg.Audience == "" {
			return nil, errors.New("jwtauth: audience required but not configured")
		}
		opts = append(opts, jwt.WithAudience(v.cfg.Audience))
	}
	claims := jwt.MapClaims{}
	if _, err := jwt.NewParser(opts...).ParseWithClaims(raw, claims, v.kf.Keyfunc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return claims, nil
}
