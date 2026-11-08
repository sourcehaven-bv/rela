package tenanthost

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
)

// verifiedKey is the ctx key for the identity the host verified. Unexported,
// so only the host can set it and a request cannot supply its own.
type verifiedKey struct{}

// verified is the host's verification of one assertion. It is keyed on the
// raw token, so a reuse answers only for the exact token that was verified.
type verified struct {
	raw string
	id  dataentry.AssertedIdentity
}

func withVerified(ctx context.Context, raw string, id dataentry.AssertedIdentity) context.Context {
	return context.WithValue(ctx, verifiedKey{}, verified{raw: raw, id: id})
}

// ReuseAssertion wraps the tenant application's verifier so its JWT gate
// reuses the host's verification of the same token instead of verifying it a
// second time. A token the host did not verify on this request, or a request
// that did not come through the host, is verified by v as usual.
//
// The reuse is safe because the host verified with the same verifier and the
// key is the raw token: a match means v would have returned the same
// identity.
//
// Nil: rejected by the caller's gate, which requires a verifier; ReuseAssertion
// itself does not check, because it is called only from wiring with the
// verifier the host was built with.
func ReuseAssertion(v AssertionVerifier) AssertionVerifier {
	return reuse{v: v}
}

type reuse struct{ v AssertionVerifier }

func (r reuse) VerifyAssertion(ctx context.Context, raw string) (dataentry.AssertedIdentity, error) {
	if m, ok := ctx.Value(verifiedKey{}).(verified); ok && m.raw == raw {
		return m.id, nil
	}
	return r.v.VerifyAssertion(ctx, raw)
}

// errOtherOrg is the refusal of an identity or webhook for another org.
var errOtherOrg = errors.New("tenanthost: verified for another organization")

// ForOrg wraps a tenant application's verifier so it accepts only
// identities of orgID.
//
// The host already routes every request by its verified org, so this
// refuses nothing today. It is the tenant's own check, so a second entry
// point that reached a tenant app without the host could not act in it as
// another org's user, which provisioning would otherwise accept.
//
// Nil: rejected by the caller's gate, as for [ReuseAssertion].
func ForOrg(v AssertionVerifier, orgID string) AssertionVerifier {
	return forOrg{v: v, org: orgID}
}

type forOrg struct {
	v   AssertionVerifier
	org string
}

func (f forOrg) VerifyAssertion(ctx context.Context, raw string) (dataentry.AssertedIdentity, error) {
	id, err := f.v.VerifyAssertion(ctx, raw)
	if err != nil {
		return dataentry.AssertedIdentity{}, err
	}
	if id.OrgID != f.org {
		return dataentry.AssertedIdentity{}, errOtherOrg
	}
	return id, nil
}

// WebhookForOrg is [ForOrg] for a tenant application's IdP webhook
// receiver: it accepts only webhooks whose verified body names orgID.
func WebhookForOrg(v WebhookVerifier, orgID string) WebhookVerifier {
	return webhookForOrg{v: v, org: orgID}
}

type webhookForOrg struct {
	v   WebhookVerifier
	org string
}

func (f webhookForOrg) VerifyWebhook(ctx context.Context, raw string) (dataentry.WebhookClaims, error) {
	claims, err := f.v.VerifyWebhook(ctx, raw)
	if err != nil {
		return dataentry.WebhookClaims{}, err
	}
	if claims.OrgID != f.org {
		return dataentry.WebhookClaims{}, errOtherOrg
	}
	return claims, nil
}
