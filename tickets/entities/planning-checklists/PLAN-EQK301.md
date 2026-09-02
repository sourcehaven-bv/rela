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

**Scope: RESCOPED mid-implementation.**

Originally specified as a graph query (`person where status = 'active'`) with a
literal fallback. Rescoped by the project owner to domains and literals, with
the query form deferred.

The trigger was an architectural finding rather than a change of mind: resolving
a query needs `internal/filter`, and `.go-arch-lint.yml` withholds it from
`internal/mail` deliberately — the dependency comment there states a send script
has no graph access "by construction rather than by convention". Working around
that is a design decision, not an import fix.

And domains deliver most of the value: the threat is a script mailing an
ATTACKER-chosen address, and `*@sourcehaven.nl` stops that without the config
needing to know which people currently exist. The query form's advantage —
tracking the graph as staff join and leave — governs WHO inside the org gets
mail, not whether mail leaves the org.

IN:

```yaml
recipients:
  also_allow:
    - "*@sourcehaven.nl"     # whole-domain pattern
    - "ops@example.com"      # literal
  # or
  allow_any: true
```

OUT, deferred: the `query:` / `property:` form, pending an answer to the
arch-lint question — either resolve outside `mail` and pass the address set in,
or widen the boundary with a stated reason.

OUT: backwards compatibility, waived by the project owner, consistent with
TKT-JVHSOZ.

**Acceptance Criteria:**

1. An absent `recipients:` block DENIES every address.
*Test:* zero policy, any address → error naming the config, nothing reaches the
transport.
2. A literal entry matches its address.
3. A `*@domain` pattern matches any address at that domain.
4. A domain pattern is NOT a suffix match.
*Test:* `attacker@evil-example.com` must not match `*@example.com`.
5. `allow_any: true` permits everything.
6. Matching is case-insensitive.
7. A denial names the refused address but NOT the allowed set.
8. A sender that declares no policy denies.
9. The operator's block reaches the binding.

AC1 is the load-bearing one. AC4 is the security-critical one — a suffix test is
the classic allowlist bypass. AC8 and AC9 exist because the enforcement is only
as good as its plumbing: a correct check nothing feeds is inert.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**

- The optional-interface pattern (`RecipientPolicyCarrier`) follows
`NotFoundError` in the same package: the wiring site injects a value and this
package asks it a question it may or may not be able to answer. Widening
`MailSender` would force every implementation and test double to carry a policy
they have no opinion about.
- The parse/enforce split follows the arch-lint boundary rather than fighting
it: `internal/mail` parses and validates, `internal/lua` enforces.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`mail.RecipientConfig` parses the block and converts to a `lua.RecipientPolicy`.
`LuaSender` carries that policy and satisfies `lua.RecipientPolicyCarrier`.
`luaMailSend` asks the sender for it and checks before handing anything to the
transport.

**Alternatives considered:**

- *Pass the policy as a Runtime option.* Implemented first, then REJECTED in
favour of the carrier. The sender is already the thing built from `mail.yaml`; a
second wiring step is a second thing to forget, and forgetting it fails OPEN
unless the default is deny. Carrying it on the sender makes "configured the
transport" and "configured the recipients" the same act.
- *Widen `MailSender` with a policy method.* Rejected: every test double and
every future transport would have to carry a policy it has no view on.

**Files to modify:** `internal/mail/config.go`, `internal/mail/script.go`,
`internal/lua/mailrecipients.go`, `internal/lua/mail.go`, plus tests and
`docs-project/entities/guides/GUIDE-mail.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `recipients:` is operator config, validated at LOAD so a typo surfaces from
`rela validate` rather than from the first message that fails to send.
- `to` is script-supplied and is what this bounds.
- An ALLOWLIST, not a blocklist: a partial wildcard (`ops-*@x.com`) is refused
at load rather than matched loosely, because every extra wildcard position is
another way to write a pattern admitting more than the operator pictured.

**Security-Sensitive Operations:**

Three fail-closed defaults, each needing its own test because each is reached by
a different route: an absent block (AC1), a sender that declares no policy
(AC8), and a partial wildcard (refused at load rather than silently literal).

The denial message names the refused address — the operator needs it, and it is
not a credential — but never the allowed SET. One denied send must not hand a
script every address on the allowlist (AC7).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one per acceptance criterion, driven through `mail.send`
rather than the checker, so the wiring is covered along with the logic.

**Negative Tests:** AC1, AC4, AC7 and AC8 are all negatives, and they are the
point. A positive-only suite would pass against a policy that permits
everything.

**Mutation plan** — each must redden only its own case:

1. unconfigured permits → AC1 reddens
2. suffix match instead of domain match → AC4 reddens
3. non-carrier sender permits → AC8 reddens
4. `LuaSender` stops carrying the policy → AC9 reddens

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *The check is correct but nothing feeds it.* The real risk for a control
split across two packages, and it MATERIALISED: the enforcement was complete and
inert for a while, because no sender carried a policy. Caught by asking "what
implements this interface?" rather than by a test. Now covered by AC9.
- *A suffix match creeps in.* The classic bypass. Pinned by AC4.
- *Existing mail tests mask the default.* They all send without a policy, so
deny-by-default would fail them. They now wrap their sender in an explicit
allow-any, which keeps them testing the send path — and AC1 still fails if the
default flips.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `GUIDE-mail.md` — a new config key with a deny-by-default that inverts
the file's usual convention. That inversion has to be stated, or an operator
reads the first denial as a bug.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the carrier-versus-option question was settled
during implementation rather than before it — see Alternatives. The deciding
argument is that the carrier makes the policy travel with the transport it was
configured beside, so there is no second wiring step to forget.
