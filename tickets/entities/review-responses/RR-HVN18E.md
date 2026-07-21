---
id: RR-HVN18E
type: review-response
title: AC10 decision written against isUnstamped behaviour that does not exist; wrong fix is a broad authz regression
finding: 'The ticket''s unmatched-principal decision and the plan claim an unmatched verified principal is currently hard-denied by isUnstamped. This is factually wrong: ResolvePrincipal returns ("", nil) on no-match (acl/declarative.go:141-142), resolvePrincipalEntity returns ctx unchanged (dataentry/router.go:277-278), so Principal.User stays the verified sub and isUnstamped returns false (acl/request.go:190-196). The request already proceeds today with zero roles. AC10 needs no code change — but the plan invites an implementer to relax isUnstamped, which is the only gate catching entry points that forgot to stamp identity, for every path. That would be a broad silent authz regression.'
severity: critical
resolution: 'Confirmed against source and the ticket''s decision section rewritten. Verified trace: ResolvePrincipal returns ("", nil) at declarative.go:141-142; resolvePrincipalEntity returns ctx unchanged at router.go:277-278; Principal.User stays the verified sub so isUnstamped returns false at request.go:190-196. The pre-existing comment at router.go:265-268 documents this as intended ("a principal absent from the graph is expected, e.g. a break-glass identity"). AC10 therefore requires no code change and the plan no longer implies one. isUnstamped is explicitly OUT OF SCOPE and must not be touched — stated as such on the ticket so an implementer cannot read scope into it. Two tests added: (a) unmatched verified principal proceeds with asserted roles only, pinning the behaviour so it cannot drift; (b) a Principal with asserted roles but blank Tool is still rejected, guarding the fail-closed gate against exactly the bad fix this finding warns about. The sub-sanitizes-to-"unknown" hazard (isBlankOrUnknown, request.go:199-202) is recorded as a known pre-existing edge with a test, not fixed here.'
status: addressed
---

## Finding

The ticket's unmatched-principal decision and the plan's Edge Cases both claim
an unmatched verified principal is "currently hard-denied by `isUnstamped`".
**This is factually wrong.** Traced:

- `ResolvePrincipal` returns `("", nil)` on no-match (`acl/declarative.go:141-142`).
- `resolvePrincipalEntity` hits `if id == "" || id == p.User { return ctx }`
(`dataentry/router.go:277-278`) and returns ctx **unchanged** — no re-stamp.
- `Principal.User` therefore remains the verified `sub`: non-empty and not
`"unknown"`, so `isUnstamped` (`acl/request.go:190-196`) returns **false**.
- `ForPrincipal` succeeds; the request proceeds today with zero global roles
(`walkMembers` returns `{sub}`, `Assignments[sub]` misses).

The existing doc comment at `router.go:265-268` states this intent outright: "a
principal absent from the graph is expected, e.g. a break-glass identity
assigned by raw UPN".

## Impact

Two directions, the second serious:

1. **AC10 requires no code change.** The decision the user made is already the
system's behaviour.
2. **The plan invites a dangerous non-change.** An implementer reading "must
keep hard-denying … the change here is that a verified principal is not
unstamped merely because its subject has no entity" will go relax `isUnstamped`.
That gate is the only thing catching an entry point that forgot to stamp
identity, and its 500 (`router.go:229-240`) is load-bearing for **every** path,
not just the asserted one. Relaxing it is a broad, silent authz regression.

## Resolution

Rewrite the decision section against what the code actually does. State
explicitly: **`isUnstamped` is not to be touched by this ticket.** Add a test
pinning "unmatched verified principal proceeds with asserted roles only,
`isUnstamped` unchanged" so the absence of a change is deliberate and durable.

Secondary hazard worth a test: a verified `sub` that sanitises to the literal
`"unknown"` is treated as unstamped by `isBlankOrUnknown` (`request.go:199-202`)
→ 500. Pre-existing, but asserted roles make it worse — a principal with valid
cryptographic grants gets a 500 instead of its grants.
