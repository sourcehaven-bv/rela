---
id: PLAN-3ULDLM
type: planning-checklist
title: 'Planning: idp-sync example: validate webhook claims before interpolating them'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `examples/idp-sync.lua` interpolates `org_id` and `user_id` from
the webhook JWT straight into an outbound request path and an entity query, with
no validation. GitHub issue #1083 (IB-review rela#1069), severity Low.

**Scope — IN:** allowlist validation of both identifiers before either
interpolation, with a comment explaining the reasoning and how to widen it.

**Scope — OUT:**

- Anything in rela core. This is an example script; the JWT verification it
depends on is core and is not in question.
- A general Lua string-escaping helper. Tempting, but this is one script with two
interpolation sites, and a shared helper would need a policy on what "safe"
means for paths versus filters versus bodies.

**Acceptance Criteria:**

1. Identifiers containing `/`, `?`, `#`, whitespace or newlines are rejected
before reaching either interpolation site.
2. Ordinary identifier shapes (UUID, ULID, email, slug) are accepted.
3. The rejection uses the script's existing structured error return, not a new
failure mechanism.

## Research

- [x] For larger features: run `/research`
- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — a five-line guard in an example.

**Existing Solutions:** the script already has two guard clauses in the same
shape (missing secrets, missing params), each returning `{ message_type =
"error", message = ... }`. The new check follows them so the script has one
failure idiom rather than two.

The project's design-review guidance is explicit that an allowlist beats a
blocklist here — its own worked example is *"Use allowlist (alphanumeric +
hyphen + underscore) instead of blocklist"*.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** a `valid_id` helper matching `^[%w._@%-]+$`, applied to
both identifiers immediately after the existing empty-check.

**Files to modify:** `examples/idp-sync.lua`.

**Alternatives considered:**

1. *Percent-encode instead of reject.* Rejected — encoding turns a malformed
identifier into a *different* valid one and the sync silently targets the wrong
record. Rejecting is louder and correct.
2. *Only document the risk, per the issue's second suggestion.* Rejected — the
issue offers "validate or document", and validation is five lines. A comment
telling the reader to add validation is worse than the validation.
3. *Blocklist the dangerous characters.* Rejected — see the project's own
guidance; the next unexpected character is the one you did not list.

**Dependencies:** none — plain Lua patterns, no library.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `org_id` / `user_id` from the verified webhook
JWT. Validation is an **allowlist** on the whole string, anchored at both ends.

**Security-Sensitive Operations:** two interpolation sites — an outbound
operator-API request path, and an entity filter string. Both are now unreachable
with an identifier outside the set.

*What this is and is not.* Defence in depth, not the primary control: the JWT is
cryptographically verified (ES256, confused-deputy guard via a separate
audience), which is why the finding is Low. The comment says so, so nobody reads
the regex as the protection and relaxes the verification. What it defends
against is a compromised or misconfigured IdP emitting an unexpected subject —
the case where the primary control is intact but its output is not what was
assumed.

**Error handling:** the message names the failing condition without echoing the
offending value, so a hostile identifier cannot be reflected into an operator's
logs or UI.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** the repo has no test harness for `examples/`, so the pattern
was executed against a real `lua` interpreter over a twelve-case table rather
than reasoned about. A Lua character class is easy to get subtly wrong, and a
too-narrow one silently rejects legitimate users — a failure mode that would
only surface in someone else's production.

**Edge Cases:** path traversal (`../etc/passwd`), path separator, query and
fragment characters, space, newline, empty, and the realistic `auth0|abc123` —
which the default set rejects, deliberately, with the comment naming it.

**Negative Tests:** the six rejection cases in that table.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

| Risk | Mitigation |
|---|---|
| Pattern too narrow → legitimate subjects rejected in someone's deployment | Verified against real identifier shapes; the comment names Auth0's format and says how to widen |
| Someone widens it to `.*` to make an error go away | The comment says to widen character by character and never to `.*`, and says what the point is |
| Read as *the* control, leading to relaxed JWT verification | The comment states it is defence in depth and names the primary control |

**Effort:** xs

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] The comment in the script itself IS the documentation — this is example
code, where the surrounding prose is the deliverable as much as the code.
- [x] ~~docs/ guides~~ (N/A: no rela behaviour changes; the example's own header
already documents its params and secrets.)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: five lines in
an example, with the allowlist-over-blocklist decision already made by the
project's own guidance. The one judgement call — reject rather than encode — is
recorded under Alternatives.)
- [x] All critical/significant findings addressed in plan — none raised.

**Design Review Findings:** N/A.
