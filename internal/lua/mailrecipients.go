// Recipient allowlist for mail.send (TKT-USQNA3).
//
// mail.send's `to` field is chosen entirely by the script. This file is the
// operator's say in the matter: an address is delivered to only if the
// operator declared it, and the declaration lives in `.rela/mail.yaml` under
// `recipients:` rather than anywhere a script can reach.
//
// # Deny by default, and deliberately so
//
// An ABSENT `recipients:` block denies everything. That is the opposite of how
// every other absent mail setting behaves — no `mail.yaml` at all means "mail
// is off", a missing `port` means 587 — and the asymmetry is the point.
//
// The two failure modes are not comparable. Permitting on absence fails
// silently and irreversibly: mail leaves the ACL perimeter, and nobody
// discovers the leak until the recipient replies. Refusing on absence fails
// loudly and harmlessly: the script gets a typed error naming the exact config
// key it needs, and the operator adds four lines of YAML. A control whose
// unconfigured state is "allow" is not a control.
//
// So the missing block is read as "not yet decided, so refuse", never as
// "unconfigured, so permit".
//
// # Why the gate lives in internal/lua
//
// It has to. `.go-arch-lint.yml` grants internal/mail only mailrender,
// secrets, lua and principal, and says why on the grant itself: a send script
// is built with a ZERO ReadDeps so it "has no graph access at all, by
// construction rather than by convention". Resolving `person where status =
// 'active'` needs store, filter and metamodel — precisely the three that grant
// withholds.
//
// internal/lua already holds all three, already owns the [EntityReader] seam
// that IS the read-ACL (DEC-O59WM4), and is where the recipient list enters
// the system in the first place. So internal/mail PARSES the block (it owns
// mail.yaml) and this package ENFORCES it. The parsed policy crosses between
// them as [RecipientPolicy], a plain data type declared here at the consumer.
package lua

import (
	"errors"
	"fmt"
	"strings"
)

// RecipientPolicy is the operator's declared recipient set, parsed from
// `.rela/mail.yaml`.
//
// Declared HERE rather than imported from internal/mail, per CLAUDE.md's
// interfaces-at-the-call-site rule and for the same reason [MailSender] is:
// internal/mail depends on this package, so the arrow can only point one way.
// internal/mail builds one of these from its YAML and hands it over.
//
// The ZERO VALUE DENIES EVERYTHING, and that is load-bearing rather than
// incidental. A wiring site that forgets to carry the policy, or a config
// struct that gains a field nobody populated, produces a gate that refuses —
// never one that waves everything through. Same posture as [Capabilities],
// applied to a different axis.
type RecipientPolicy struct {
	// Configured records that an operator actually wrote a `recipients:`
	// block.
	//
	// It exists to distinguish "the operator declared an empty set" from "the
	// operator declared nothing", which the other fields cannot: both are the
	// zero value, both deny, but they need DIFFERENT ERRORS. One says "add a
	// recipients: block to .rela/mail.yaml"; the other says "this address is
	// not in the set you declared". An operator handed the wrong one of those
	// looks in the wrong place.
	Configured bool

	// AllowAny is the escape hatch: every address is permitted and nothing is
	// resolved.
	//
	// It must be a deliberate `allow_any: true` in the file. It is never
	// defaulted, never inferred from an empty block, and never reached by
	// omission — a deployment that has decided this constraint is not for
	// them says so in one line, and that line is greppable in a config review.
	AllowAny bool

	// AlsoAllow holds literal addresses that are NOT entities — an ops alias,
	// an external auditor's mailbox — unioned with the query result.
	//
	// Already normalized by [NormalizeRecipient] when internal/mail builds
	// this, so the matcher compares like with like rather than normalizing
	// the same constant on every send.
	AlsoAllow []string
}

// RecipientPolicyCarrier is an OPTIONAL capability a [MailSender] may
// implement to declare which recipients the operator permitted.
//
// Optional-interface rather than a widened MailSender or a new
// MailSenderLoader return value, following [NotFoundError]: the wiring site
// injects a value, and this package asks it a question it may or may not be
// able to answer. Widening MailSender would force every implementation and
// every test double to carry a policy they have no opinion about.
//
// A sender that does NOT implement this denies everything, exactly as an
// absent block does. The unimplemented case and the unconfigured case are the
// same fact — nobody declared a recipient set — so they get the same answer.
type RecipientPolicyCarrier interface {
	// RecipientPolicy returns the operator-declared recipient set.
	RecipientPolicy() RecipientPolicy
}

// errRecipientsNotConfigured is the denial when no `recipients:` block exists.
//
// It names the file and the key because that is the entire remedy. An operator
// reading "recipient not allowed" with no further detail has to go find out
// which of several ACL surfaces refused them; this one says where to type.
var errRecipientsNotConfigured = errors.New(
	"mail.send: no recipients are configured, so every address is refused — " +
		"add a `recipients:` block to .rela/mail.yaml naming who may be mailed " +
		"(`query` + `property` to select them from the graph, `also_allow` for " +
		"literal addresses, or `allow_any: true` to disable this check)")

// recipientDeniedKind is the err.kind a script sees when an address is refused.
//
// A FOURTH kind, deliberately not a reuse of not_configured. The two facts are
// different and a script branches on them differently:
//
//   - not_configured — the project has no mail transport. Nothing will send,
//     and a script that mails an optional digest should skip it entirely.
//   - recipients_not_allowed — mail works fine; THIS address is not permitted.
//
// Collapsing them would make a script that feature-detects not_configured
// ("mail is off, skip the digest") silently skip the digest on a perfectly
// healthy deployment because one address fell outside the allowlist. The
// operator would see no error and no mail.
const recipientDeniedKind = "recipients_not_allowed"

// recipientPolicyFor extracts the policy a sender declares.
//
// A sender that cannot answer yields the zero policy, which denies. This is
// the fail-closed default doing its job at the seam: a transport written
// before this feature, or a test double that never thought about it, must not
// be read as an operator's blessing.
func recipientPolicyFor(sender MailSender) RecipientPolicy {
	if carrier, ok := sender.(RecipientPolicyCarrier); ok {
		return carrier.RecipientPolicy()
	}
	return RecipientPolicy{}
}

// checkRecipients reports whether every address in `to` is permitted.
//
// WHOLE-SEND, not per-address, and that is the API's most important property.
// It takes the entire recipient list so the query is resolved ONCE per send
// however many addresses there are: a fan-out to 200 people is one graph scan,
// not two hundred. Handing this function a single address in a loop would
// reintroduce exactly the cost the ticket set out to avoid, which is why there
// is no single-address entry point to reach for.
//
// The resolved set is NOT retained past this call. Caching it on the Runtime
// was considered and rejected: a scheduler task or automation can hold a
// runtime for minutes, and a cached set would keep mailing someone for minutes
// after the operator marked them inactive — the very drift a query-based
// allowlist exists to eliminate, reintroduced silently and with a lifetime
// nobody chose. So the unit of consistency is ONE SEND: within a single
// mail.send the set is frozen (a fan-out cannot half-apply a change), and
// between two sends a graph change is observed by the second. That matches the
// question an operator actually asks — "was this message allowed when it went
// out?" — and costs one ListEntities scan per send, the same order the
// scheduled fan-out already pays per occurrence and negligible beside the SMTP
// round-trip it precedes.
//
// ALL OR NOTHING: one denied address refuses the whole send. Delivering to the
// permitted subset was the alternative, and it is worse — a script that
// believed it mailed five people would have mailed three and been told it
// succeeded. A partial send reported as success is a harder bug to find than a
// refused one.
//
// Nil: a nil reader or metamodel DENIES; it never falls back to an ungated
// read or an empty-and-therefore-permissive set (RR-X9NVHI).
func checkRecipients(policy RecipientPolicy, to []string) error {
	if policy.AllowAny {
		return nil
	}
	if !policy.Configured {
		return errRecipientsNotConfigured
	}

	for _, addr := range to {
		if !policy.permits(addr) {
			// The refused address IS echoed: it is not a credential, and it is
			// the one fact the operator needs. The allowed SET is not — one
			// denied send must not enumerate every permitted address.
			return fmt.Errorf(
				"mail.send: recipient %q is not in the configured recipients allowlist; "+
					"add it or a `*@domain` pattern to `recipients.also_allow` in .rela/mail.yaml",
				addr)
		}
	}
	return nil
}

// permits reports whether addr matches any entry: a literal address, or a
// whole-domain `*@example.com` pattern.
//
// Linear over AlsoAllow rather than a map, because the list is operator-written
// config — a handful of entries — and half of them are patterns that a map
// lookup could not answer anyway. A set would buy nothing and would need the
// pattern scan beside it regardless.
func (p RecipientPolicy) permits(addr string) bool {
	norm := NormalizeRecipient(addr)
	for _, entry := range p.AlsoAllow {
		if entry == norm {
			return true
		}
		if domain, ok := strings.CutPrefix(entry, "*@"); ok {
			// Compare the address's domain, not a suffix of the whole string:
			// a suffix test would let `evil-example.com` match `*@example.com`
			// — the classic allowlist bypass.
			if at := strings.LastIndex(norm, "@"); at >= 0 && norm[at+1:] == domain {
				return true
			}
		}
	}
	return false
}

// NormalizeRecipient puts an address into the form the allowlist compares on:
// trimmed and lowercased.
//
// Case-insensitive because the operator will write the address once, in
// whichever case they happen to type, and `Ops@Example.com` in mail.yaml must
// match `ops@example.com` in a script. Domains are case-insensitive by RFC
// 1035; local-parts are technically case-SENSITIVE by RFC 5321, but no mail
// system in practice treats them so, and an allowlist that refused
// `Alice@example.com` because the graph says `alice@example.com` would be
// experienced as a bug and worked around by adding both — which is strictly
// worse than folding here.
//
// Exported so internal/mail can normalize `also_allow` once at config load
// rather than on every send, and so both sides provably use the SAME rule.
// Two independent normalizations that drift is how an allowlist starts letting
// through what it should not.
func NormalizeRecipient(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}
