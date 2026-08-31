---
id: PLAN-EQK301
type: planning-checklist
title: 'Planning: Operator-configured recipient allowlist for mail.send'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: a `recipients:` block in `.rela/mail.yaml`, DENY-BY-DEFAULT,
gating every address `mail.send` is handed. Three ways an address may be
permitted, unioned: a `query` + `property` pair resolved against the graph, a
literal `also_allow` list, and the explicit `allow_any: true` escape hatch. An
absent block denies everything with a typed `recipients_not_allowed` error
naming the missing config.

OUT:
- The `caps.Mail` capability gate (TKT-JVHSOZ, sibling branch). Different
question: that one asks "may this script send mail at all", this one asks "to
whom". They compose; neither subsumes the other.
- Backwards compatibility — waived by the project owner. There is no
transition period and no "warn once then permit" mode.
- Gating the DECLARATIVE scheduled-mail path
(`appbuild.RunScheduledTemplate`). Its recipient is already operator-declared:
`schedules.yaml` `for_each` names the entity type and filter, and
`mail-templates.yaml` names the address property. The operator has already
written the allowlist there, in a more precise form. Adding a second one would
mean maintaining the same set twice, and a mismatch would silently stop
scheduled mail. Recorded as a decision, not an omission.
- Wildcards in `also_allow` — see the Approach section for the decision and
the forward-compatibility measure taken so it can be added later without a
breaking change.

**Acceptance Criteria:**

1. No `recipients:` block at all → send DENIED. `TestMailSend_NoRecipientPolicyDenies`
2. The denial error is typed `recipients_not_allowed` and its message names
`recipients` and `.rela/mail.yaml`. `TestMailSend_DenialNamesTheMissingConfig`
3. `recipients:` present, address not in the resolved set → DENIED.
`TestMailSend_AddressOutsideQueryResultDenied`
4. Address in the query result → allowed. `TestMailSend_AddressInQueryResultAllowed`
5. Address only in `also_allow` → allowed. `TestMailSend_AddressInAlsoAllowAllowed`
6. `allow_any: true` → any address allowed. `TestMailSend_AllowAnyPermitsEverything`
7. A multi-recipient send resolves the query exactly ONCE, not once per
address. `TestRecipientGate_ResolvesQueryOncePerSend`
8. One denied address in a multi-recipient send denies the WHOLE send; nothing
is delivered. `TestMailSend_OneDeniedAddressDeniesTheWholeSend`
9. Matching is case-insensitive on the address, because SMTP domains are and
an operator will not write both cases.
`TestRecipientGate_MatchIsCaseInsensitive`
10. A literal `*` in `also_allow` is REFUSED at config load, so a wildcard
written today fails loudly instead of silently matching nothing.
`TestConfig_AlsoAllowRejectsWildcard`
11. A `recipients:` block that configures nothing usable (no `query`, no
`also_allow`, no `allow_any`) is refused at load rather than becoming a
deny-everything block that looks configured.
`TestConfig_EmptyRecipientsBlockRejected`
12. A query naming an unknown entity type, or a graph read that fails, DENIES
— it never falls open. `TestRecipientGate_ResolveFailureDenies`

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: the mechanism is entirely
in-repo — an existing filter DSL, an existing reader seam, an existing error
convention. No third-party library is involved.)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — no new dependency.

**Existing Solutions:**
- `internal/appbuild/scheduler_foreach.go` `ScheduledForEachEntities` is the
closest prior art and the model for the resolver: `filter.ParseAll` over a list
of expressions, `VisibleReader.ListEntities(store.EntityQuery{Type:...})`, then
`filter.MatchAll` per row against the metamodel's entity def. Reusing that shape
means the allowlist query and the scheduled fan-out select entities by the same
rules, so an operator does not learn two dialects.
- `internal/filter` is the project's query-filter DSL (`status=active`,
globs, `=~`, ranges). There is NO SQL-ish `"person where status = 'active'"`
parser anywhere in the repo — verified by search. So the ticket's example syntax
is a NEW surface if taken literally; see the Approach section for how it is
reconciled.
- `internal/lua/deps.go` `EntityReader` is the consumer-side read seam, and
which implementation the wiring site puts there IS the read-ACL (DEC-O59WM4).
The gate reads through it and therefore inherits ACL for free rather than
inventing a second visibility rule.
- `internal/lua/mail.go` `pushMailError` and `classifyMailError` establish the
`kind`/`message`/`retry_after`/`details` error table and the `not_configured`
kind a script feature-detects on. The new denial follows it exactly.
- `internal/lua/deps.go` `NotFoundError` is the precedent for an OPTIONAL
capability interface declared consumer-side that an injected value may
implement. That is how the policy travels from `internal/mail` into
`internal/lua` without widening the shared `MailSenderLoader` signature.
- `mail.Config.Capabilities` (TKT-YH52OM) is the precedent for a fail-closed
YAML block whose zero value grants nothing. `recipients:` is the same shape
applied to a different axis.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Where the gate lives — and why it cannot live in `internal/mail`.**
`.go-arch-lint.yml` grants `internal/mail` only `mailrender`, `secrets`, `lua`
and `principal`, and the comment on that grant says why in as many words: the
send runtime is built with a ZERO `ReadDeps` so "a send script has no graph
access at all, by construction rather than by convention". A query resolver
inside `internal/mail` would need `store`, `filter` and `metamodel`, which would
undo exactly the property that comment is defending.

`internal/lua` already has all three, already holds the `EntityReader` seam, and
is where the recipient list actually enters the system. So the split is:

- `internal/mail` PARSES the config, because it owns `mail.yaml`, and
validates it. It resolves nothing.
- `internal/lua` ENFORCES, because it is the only side with a graph.

The parsed policy travels between them as `lua.RecipientPolicy`, a plain data
type declared consumer-side in `internal/lua`, surfaced through an OPTIONAL
one-method interface (`lua.RecipientPolicyCarrier`) that `mail.LuaSender`
implements. `internal/mail` already imports `internal/lua`, so this needs no new
arch edge and no change to `MailSenderLoader`'s signature — which matters
because the sibling ticket is editing the same files.

**Query syntax — reconciling the ticket's example.** The ticket writes `query:
"person where status = 'active'"`. No such parser exists. Rather than invent a
second query dialect for one config key, `parseRecipientQuery` accepts that
exact shape and lowers it onto the existing pieces: the leading word is the
entity type, everything after the `where` keyword is a list of `internal/filter`
expressions split on `and`, and single or double quotes around a value are
stripped. `person` alone (no `where`) is legal and means every entity of that
type. This gives the operator the syntax the ticket specifies while the matching
semantics remain `internal/filter`'s, so the allowlist query and
`schedules.yaml`'s `for_each` agree about what `status=active` means.

**DECISION 1 — Query cost: resolved once per send, cached for the send only.**
`RecipientGate.Allow(ctx, addresses)` takes the WHOLE recipient list and
resolves the query at most once for that call, no matter how many addresses it
holds. A 200-way fan-out is one graph scan.

The resolved set is NOT cached beyond the individual send. The alternative —
memoizing on the Runtime for the life of the script — was considered and
rejected. A long-running script (a scheduler task, an automation) can hold a
runtime for minutes, and a per-runtime cache would keep mailing a person for
minutes after the operator set their status to `inactive`. That is precisely the
drift the query-based allowlist exists to eliminate; a cache would reintroduce
it silently and with a duration nobody chose.

The mid-run edge case is therefore decided as follows: **an entity gaining or
losing `status = 'active'` between two `mail.send` calls in the same script IS
observed by the second call.** Within a single `mail.send` the set is frozen, so
a fan-out cannot half-apply a change. The unit of consistency is one send, which
is also the unit an operator reasons about ("was this message allowed when it
went out?"). The cost is one `ListEntities` scan per `mail.send` rather than per
script, which is the same order as the scheduled fan-out already does per
occurrence, and cheap next to the SMTP round-trip it precedes.

**DECISION 2 — Wildcards in `also_allow`: OUT of scope, and refused loudly.**
`also_allow` matches literal addresses only. `*@sourcehaven.nl` does not work,
and — this is the load-bearing half — a `*` anywhere in an `also_allow` entry is
REJECTED AT CONFIG LOAD with an error saying wildcards are not supported.

Refusing beats ignoring. A literal-only matcher that silently treats `*` as an
ordinary character would accept the config, match nothing, and present as "mail
mysteriously denied" — the operator's mental model would be "I allowed the
domain" while the behaviour is "I allowed an address that cannot exist".
Rejecting at load makes the gap visible at the moment it is created.

Refusing also RESERVES the syntax. The reason the ticket asks for a decision now
is that retrofitting a matcher is hard once `*` means "literal asterisk" in the
field, because turning it into a metacharacter later is then a breaking change
to a working config. With `*` refused today, adding domain wildcards later only
ever turns an error into a success — a compatible change by construction. The
reasoning is recorded on `validateAlsoAllow` so the next person finds it where
they would try to add the feature.

Why not just implement wildcards now: a domain wildcard is a strictly weaker
control than the query (`*@sourcehaven.nl` permits every address at the domain,
including ones belonging to nobody in the graph and ones that never existed),
and the ticket is explicit that the query is the primary mechanism because it
tracks reality. Shipping the weaker control in the same change would invite it
to be used as the default. It is one function away when a real deployment needs
it.

**DECISION 3 — Error shape: a fourth `kind`, `recipients_not_allowed`.** The
denial pushes the existing `(nil, err_table)` shape with a NEW kind rather than
reusing `not_configured`. Two distinct facts:

- `not_configured` — the project has no mail transport. Nothing will send.
- `recipients_not_allowed` — mail works; this address is not permitted.

A script that feature-detects `not_configured` to decide "mail is off, skip the
digest" would, if the denial reused that kind, silently skip the digest on a
working deployment because ONE address was outside the allowlist. Distinct kinds
keep that branch honest.

The message names both the block and the file, e.g. `recipient
"eve@evil.example" is not in the configured recipients allowlist; add it to
recipients.also_allow or widen recipients.query in .rela/mail.yaml`, and when
the block is absent entirely it says so specifically, naming `recipients:` as
the thing to add. The address IS echoed: it is not a credential, and an operator
debugging a denial needs to know which address was refused.

**Fail-closed everywhere.** A nil `VisibleReader`, a nil metamodel, an unknown
entity type, an unparseable filter and a `ListEntities` error all DENY. This
follows `ReadDeps.VisibleReader`'s own documented rule (RR-X9NVHI: a forgotten
wiring must not become a bypass) and is the whole point of the ticket.

**Whole-send atomicity.** If any one address is denied, the send is refused and
NOTHING is delivered. The alternative — deliver to the permitted subset — would
mean a script that believed it mailed five people mailed three, with a success
return. A partial send that reports success is a worse failure than a refused
one.

Rejected alternatives:
- Filtering the recipient list down to the allowed subset and sending anyway:
see above. Silent partial delivery.
- Enforcing inside `internal/mail`: impossible without undoing the
no-graph-access property that package's arch grant exists to protect.
- Widening `lua.MailSenderLoader` to return the policy: it is one of the files
the sibling ticket edits, and an optional carrier interface is the codebase's
own established pattern (`lua.NotFoundError`) for exactly this.
- Caching the resolved set on the Runtime: see Decision 1.
- A `deny:` list beside `also_allow`: a denylist under an allowlist is
unreachable code — anything not allowed is already denied.

**Files to modify:**
- `internal/mail/config.go` — `RecipientConfig` type, field on `Config`,
validation, `RecipientPolicy()` conversion. (SHARED with TKT-JVHSOZ.)
- `internal/mail/script.go` — `LuaSender` carries the policy and exposes it.
- `internal/lua/mailrecipients.go` (NEW) — `RecipientPolicy`,
`RecipientPolicyCarrier`, `recipientGate`, query parsing, resolution.
- `internal/lua/mail.go` — the gate call in `luaMailSend`, plus the new kind.
(SHARED with TKT-JVHSOZ.)
- `internal/lua/mailrecipients_test.go` (NEW), `internal/lua/mail_test.go`,
`internal/mail/config_test.go`, `internal/mail/script_test.go`
- `docs-project/entities/guides/GUIDE-mail.md` + `just docs`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `.rela/mail.yaml` `recipients:` — operator-authored. `query` must parse into
a known entity type plus valid filter expressions; `property` must be non-empty
when `query` is set; `also_allow` entries must be non-empty, must not contain
`*`, and must not contain CR/LF/NUL (they end up in an SMTP envelope). A block
that is present but configures nothing is refused rather than silently denying
everything.
- The Lua `to` argument — untrusted by assumption; that is what the gate is
for. It is normalized (trim + lowercase) for comparison only; the address
delivered is the one the script wrote, unchanged, so the gate cannot rewrite a
destination.
- Graph property values — a `person.email` that is not a string, or is empty,
contributes NOTHING to the allowed set. It is skipped, not coerced: a numeric or
nil property silently becoming the empty string would make `to = ""` allowable.

**Security-Sensitive Operations:**
- The graph read runs through `ReadDeps.VisibleReader`, so the allowed set is
bounded by what the runtime's identity may see. A script cannot widen its own
allowlist by reading entities it is not permitted to read. Noted as a
consequence, not a claim of concealment: the operator's allowlist is config, and
config is not a secret (CLAUDE.md).
- Every resolution failure denies. There is no path where an error produces an
empty-but-permissive set.
- The denial message echoes the refused address and the config key names.
Neither is a credential; both are what the operator needs. It never echoes the
resolved allowlist, which would turn one denied send into an enumeration of
every active person's address.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1–AC12 each map to the named test listed under Acceptance
Criteria. The `internal/lua` tests drive real Lua source through a real
`Runtime` against a fake `EntityReader` holding real `entity.Entity` values and
a real `metamodel.Metamodel`, so the filter and metamodel interaction is
exercised rather than stubbed. The `internal/mail` tests drive `LoadConfig` over
real YAML on disk, so the operator-facing surface is what is asserted.

**Edge Cases:**
- Address differing only in case from the graph value, and in the domain only.
- Surrounding whitespace in `also_allow` and in the graph property.
- A `person` with no `email` property at all, and one whose `email` is a
number, a boolean, and an empty string.
- `query` with no `where` clause (whole type).
- `where` with several `and`-joined conditions.
- Quoted and unquoted values in the query.
- `allow_any: true` set together with a `query` — allow_any wins and the
query is not resolved at all (asserted by a reader that fails the test if
called).
- Zero entities matching the query, with `also_allow` non-empty: the
`also_allow` addresses still work.
- Duplicate addresses in one send.
- 200 recipients in one send: exactly one `ListEntities` call.

**Negative Tests:**
- Absent `recipients:` block → denied, error names the config (the
mutation-checked case).
- Address absent from both the query result and `also_allow` → denied.
- `also_allow` containing `*@example.com` → config load fails.
- `recipients:` present but empty → config load fails.
- `query` naming an unknown entity type → denied at send.
- `query` whose filter does not parse → config load fails.
- `property` empty while `query` is set → config load fails.
- `VisibleReader` nil → denied, not permitted.
- `ListEntities` returning an error → denied.
- A denied address in position 3 of 5 → whole send denied, sender never called.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- **Every existing `mail.send` test breaks.** Certain, not a risk — deny-by-
default is the feature. Mitigated by making the test helper explicit about which
policy it wires, so each existing test states the policy it is running under
rather than inheriting a permissive default. A helper that defaulted to
`allow_any` would hide the very regression the ticket exists to catch.
- **Merge conflict with TKT-JVHSOZ** on `internal/mail/config.go` and
`internal/lua/mail.go`. Real and expected. Mitigated by keeping both edits
additive and localized: one struct field plus one `switch` arm in config.go, one
guarded block in `luaMailSend`. `MailSenderLoader`'s signature is deliberately
untouched.
- **`Runtime` is at its plimsoll load line** (`max-methods=61`). Mitigated by
putting the gate in its own type in its own file, reached by a free function.
`Runtime` gains one field and no methods.
- **The query syntax is a new surface.** Mitigated by lowering it onto
`internal/filter` rather than implementing a parser: the accepted grammar is
deliberately tiny (`TYPE [where EXPR [and EXPR]...]`) and every semantic
question is answered by the existing DSL.
- **An operator locks themselves out.** A deployment that upgrades without
adding `recipients:` loses all script-sent mail. This is the intended behaviour
and the reason the error names the config; the guide documents the migration in
one line of YAML.

**Effort:** m (as filed).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs-project/entities/guides/GUIDE-mail.md` — a `recipients:` section
covering the deny-by-default rule, the three mechanisms, the escape hatch, the
wildcard decision, and a troubleshooting entry for the denial message.
`docs/mail.md` is GENERATED; regenerate with `just docs`.
- [x] Code documentation — the three decisions recorded at the declarations
they govern, not in a design doc that drifts.
- [x] ~~CLAUDE.md~~ (N/A: no repository-wide convention added; the work
follows the existing consumer-side-interface, fail-closed and single-load-point
rules)
- [x] ~~README~~ (N/A: the mail guide is the public reference)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the ticket
was filed with the scope, the deny-by-default rule, the five verification cases
and three named open decisions. Those three are decided above with their
reasoning and their rejected alternatives, which is the output a design review
would have produced.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** The ticket's three named decisions — query cost,
wildcards, error shape — are decided in the Approach section above, each with
the alternative that was rejected and why.
