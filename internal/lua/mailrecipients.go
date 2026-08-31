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
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
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

	// EntityType and Filters are the resolved form of the `query` key:
	// `person where status = 'active'` becomes EntityType "person" and one
	// filter. Empty EntityType means no query was configured, which is legal
	// as long as AlsoAllow or AllowAny carries the policy.
	EntityType string
	Filters    []*filter.Filter

	// Property is the entity property holding the address, e.g. `email`.
	Property string

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
func checkRecipients(
	ctx context.Context, policy RecipientPolicy, to []string,
	reader EntityReader, meta *metamodel.Metamodel,
) error {
	if policy.AllowAny {
		// Short-circuits BEFORE resolution: `allow_any: true` alongside a
		// `query` must not pay for a graph scan whose answer cannot matter.
		return nil
	}
	if !policy.Configured {
		return errRecipientsNotConfigured
	}

	allowed, err := resolveRecipients(ctx, policy, reader, meta)
	if err != nil {
		// A resolution failure DENIES. The alternative — treat an unresolvable
		// query as an empty allowlist and carry on with also_allow — would
		// turn a broken metamodel or an unreachable store into a quietly
		// narrower policy, and a broken reader into a quietly wider one if the
		// error ever moved. Refusing keeps the failure visible.
		return fmt.Errorf("mail.send: cannot resolve the configured recipients allowlist: %w", err)
	}

	for _, addr := range to {
		if _, ok := allowed[NormalizeRecipient(addr)]; !ok {
			// The refused address IS echoed: it is not a credential, and it is
			// the one fact the operator needs. The allowed SET is not — one
			// denied send must not enumerate every active person's address.
			return fmt.Errorf(
				"mail.send: recipient %q is not in the configured recipients allowlist; "+
					"add it to `recipients.also_allow` or widen `recipients.query` in .rela/mail.yaml",
				addr)
		}
	}
	return nil
}

// resolveRecipients builds the permitted address set: the query result unioned
// with the literal also_allow entries.
//
// Returns a set rather than a slice because the caller does N lookups against
// it and the graph side can be large; a linear scan per recipient would make a
// 200-way fan-out quadratic for no reason.
func resolveRecipients(
	ctx context.Context, policy RecipientPolicy,
	reader EntityReader, meta *metamodel.Metamodel,
) (map[string]struct{}, error) {
	allowed := make(map[string]struct{}, len(policy.AlsoAllow))
	for _, addr := range policy.AlsoAllow {
		allowed[addr] = struct{}{}
	}

	if policy.EntityType == "" {
		// No query configured. Legal: a deployment may allowlist a handful of
		// literal addresses and nothing else. internal/mail refuses a block
		// that configures nothing at all, so this is never a silent no-op.
		return allowed, nil
	}

	if reader == nil {
		return nil, errors.New("no entity reader is wired, so the recipients query cannot be resolved")
	}
	if meta == nil {
		return nil, errors.New("no metamodel is wired, so the recipients query cannot be resolved")
	}
	def, ok := meta.GetEntityDef(policy.EntityType)
	if !ok {
		return nil, fmt.Errorf("recipients.query names unknown entity type %q", policy.EntityType)
	}

	// Reads go through EntityReader, so the allowed set is bounded by what
	// this runtime's identity may see. A script cannot widen its own allowlist
	// by reading entities it is not permitted to read. That is a consequence
	// of the seam, not a concealment claim — the operator's allowlist is
	// config, and config is not a secret (CLAUDE.md).
	for e, listErr := range reader.ListEntities(ctx, store.EntityQuery{Type: policy.EntityType}) {
		if listErr != nil {
			return nil, fmt.Errorf("list %s: %w", policy.EntityType, listErr)
		}
		matched, matchErr := filter.MatchAll(filter.Record{
			ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.UpdatedAt,
		}, policy.Filters, def, meta)
		if matchErr != nil {
			return nil, fmt.Errorf("match %s: %w", e.ID, matchErr)
		}
		if !matched {
			continue
		}
		// A property that is missing, not a string, or empty contributes
		// NOTHING. Coercing it would be worse than skipping: a nil or numeric
		// `email` rendered as "" would put the empty string in the allowlist
		// and make `to = ""` a permitted recipient.
		raw, has := e.Properties[policy.Property]
		if !has {
			continue
		}
		text, isString := raw.(string)
		if !isString {
			continue
		}
		if norm := NormalizeRecipient(text); norm != "" {
			allowed[norm] = struct{}{}
		}
	}
	return allowed, nil
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
