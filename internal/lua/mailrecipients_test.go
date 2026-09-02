package lua

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// The recipients allowlist bounds where mail.send may deliver (TKT-USQNA3 /
// issue #1459 follow-up). The capability gate decides WHETHER a script may
// send; this decides WHERE.
//
// The load-bearing cases here are the NEGATIVES, and deny-by-default most of
// all: a configuration mistake that silently permits is the exact failure this
// prevents, and a suite that only checked the allowed cases would never catch
// it. Each negative is mutation-checked.

// policySender is a recordingMailSender that also declares a recipient policy,
// exercising the RecipientPolicyCarrier route the real senders use.
type policySender struct {
	*recordingMailSender
	policy RecipientPolicy
}

func (p policySender) RecipientPolicy() RecipientPolicy { return p.policy }

func sendTo(t *testing.T, policy RecipientPolicy, addr string) (*recordingMailSender, error) {
	t.Helper()
	rec := &recordingMailSender{}
	sender := policySender{recordingMailSender: rec, policy: policy}
	rt := NewReader(ReadDeps{}, &bytes.Buffer{},
		WithMailSender(sender),
		// NOTE: once TKT-JVHSOZ (the caps.Mail gate) merges, this needs
		// WithCapabilities(Capabilities{Mail: true}) too — the two controls
		// are independent and a send must pass BOTH.
	)
	t.Cleanup(rt.Close)
	err := rt.RunString(`
local ok, e = mail.send{to = "` + addr + `", subject = "s", text = "t"}
if not ok then error(e.message, 0) end
`)
	return rec, err
}

// An absent `recipients:` block DENIES. This is the case that matters most: it
// inverts the file's usual "absent means a sensible default" rule, because
// permitting on absence fails silently and irreversibly — mail leaves the ACL
// perimeter and nobody knows until the recipient replies.
func TestRecipients_UnconfiguredDenies(t *testing.T) {
	sender, err := sendTo(t, RecipientPolicy{}, "anyone@example.com")
	require.Error(t, err, "an unconfigured allowlist must deny")
	require.Contains(t, err.Error(), "recipients")
	require.Empty(t, sender.messages(), "nothing may reach the transport")
}

func TestRecipients_LiteralMatchAllowed(t *testing.T) {
	policy := RecipientPolicy{Configured: true, AlsoAllow: []string{"ops@example.com"}}
	sender, err := sendTo(t, policy, "ops@example.com")
	require.NoError(t, err)
	require.Len(t, sender.messages(), 1)
}

func TestRecipients_NonMatchingAddressDenied(t *testing.T) {
	policy := RecipientPolicy{Configured: true, AlsoAllow: []string{"ops@example.com"}}
	sender, err := sendTo(t, policy, "attacker@evil.test")
	require.Error(t, err)
	require.Empty(t, sender.messages())
}

func TestRecipients_DomainPatternAllowed(t *testing.T) {
	policy := RecipientPolicy{Configured: true, AlsoAllow: []string{"*@sourcehaven.nl"}}
	sender, err := sendTo(t, policy, "jeroen@sourcehaven.nl")
	require.NoError(t, err)
	require.Len(t, sender.messages(), 1)
}

// The classic allowlist bypass: a SUFFIX test would let `evil-example.com`
// match `*@example.com`. The comparison must be on the address's domain, not
// on a suffix of the whole string.
func TestRecipients_DomainPatternIsNotASuffixMatch(t *testing.T) {
	policy := RecipientPolicy{Configured: true, AlsoAllow: []string{"*@example.com"}}
	for _, addr := range []string{
		"attacker@evil-example.com",
		"attacker@notexample.com",
	} {
		t.Run(addr, func(t *testing.T) {
			sender, err := sendTo(t, policy, addr)
			require.Error(t, err, "%s must not match *@example.com", addr)
			require.Empty(t, sender.messages())
		})
	}
}

// allow_any is the escape hatch and must work, or a deployment that opted out
// of the constraint is stuck. It is separate from Configured on purpose: it
// short-circuits before any matching.
func TestRecipients_AllowAnyPermitsEverything(t *testing.T) {
	policy := RecipientPolicy{Configured: true, AllowAny: true}
	sender, err := sendTo(t, policy, "anyone@anywhere.test")
	require.NoError(t, err)
	require.Len(t, sender.messages(), 1)
}

// Address comparison is normalized, so a policy written in one case matches an
// address sent in another. Without this an operator's `Ops@Example.com` would
// silently fail to match `ops@example.com` — a denial they would read as a bug.
func TestRecipients_MatchIsCaseInsensitive(t *testing.T) {
	policy := RecipientPolicy{Configured: true, AlsoAllow: []string{NormalizeRecipient("Ops@Example.COM")}}
	sender, err := sendTo(t, policy, "ops@example.com")
	require.NoError(t, err)
	require.Len(t, sender.messages(), 1)
}

// The denial names the refused address (the operator needs it) but must not
// enumerate the allowed set — one denied send must not leak every permitted
// address to a script that only guessed one.
func TestRecipients_DenialDoesNotLeakTheAllowlist(t *testing.T) {
	policy := RecipientPolicy{Configured: true, AlsoAllow: []string{
		"secret-auditor@example.com", "cfo@example.com",
	}}
	_, err := sendTo(t, policy, "attacker@evil.test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "attacker@evil.test", "the refused address is the fact the operator needs")
	for _, secret := range policy.AlsoAllow {
		require.NotContains(t, err.Error(), secret, "the allowed set must not be enumerated")
	}
	require.NotContains(t, err.Error(), "cfo", "no allowed address may appear")
}

// A sender that does NOT implement RecipientPolicyCarrier denies everything.
//
// This is the seam's fail-closed default, and it needs its own test because
// every other case here uses a sender that DOES carry a policy — so a change
// making the non-carrier path permissive would pass the rest of this file. The
// case is real rather than theoretical: a transport written before this
// feature, or a test double that never considered it, must not be read as an
// operator's blessing.
func TestRecipients_SenderWithoutPolicyDenies(t *testing.T) {
	sender := &recordingMailSender{} // deliberately NOT a policySender
	rt := NewReader(ReadDeps{}, &bytes.Buffer{}, WithMailSender(sender))
	t.Cleanup(rt.Close)

	err := rt.RunString(`
local ok, e = mail.send{to = "anyone@example.com", subject = "s", text = "t"}
if not ok then error(e.message, 0) end
`)
	require.Error(t, err, "a sender that declares no policy must deny")
	require.Empty(t, sender.messages(), "nothing may reach the transport")
}
