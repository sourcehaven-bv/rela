---
id: TKT-USQNA3
type: ticket
title: Operator-configured recipient allowlist for mail.send
kind: enhancement
priority: medium
effort: m
status: planning
---

## Description

`mail.send`'s `to` field is entirely script-chosen with no operator constraint.
Even with the capability gate (TKT-JVHSOZ), a script that legitimately holds the
`mail` grant can address any recipient it likes.

In normal use a recipient IS a user in the system — the scheduled-mail fan-out
derives them from the graph, and no counter-example surfaced when the project
owner and I each went looking. So an operator-declared recipient set is
expressible without breaking the real use case.

## Scope

IN: `recipients:` block in `.rela/mail.yaml`, DENY-BY-DEFAULT.

```yaml
recipients:
  query: "person where status = 'active'"
  property: email
  also_allow:
    - "ops@example.com"
```

- `query` + `property` resolve against the graph, so the allowlist tracks
reality instead of drifting. This is the primary mechanism because recipients
are normally entities.
- `also_allow` carries literal addresses that are NOT entities — an ops alias, an
external auditor. Union with the query result.
- `allow_any: true` is the explicit escape hatch, for a deployment that has
decided this constraint is not for them. It must be a deliberate line in the
file, never a default.
- **Absent means DENY ALL.** A `recipients:` block that is missing is not
"unconfigured, so permit" — it is "not yet decided, so refuse". That is the
opposite of how the current code treats absent mail config, and it is
deliberate: the failure mode of permitting is a silent data leak, the failure
mode of refusing is a visible error naming the missing config.

OUT:
- The capability gate itself (TKT-JVHSOZ).
- Backwards compatibility — waived by the project owner, consistent with the
sibling ticket.

## Design notes

The denial must be a TYPED error, matching the `not_configured` convention
already in `internal/lua/mail.go`, so a script can feature-detect and an
operator gets a message naming the config rather than a generic failure.

Resolve the query ONCE per send, not per recipient: a fan-out mailing 200 people
must not run 200 queries. Consider whether the resolved set is cacheable within
a run and what invalidates it — an entity gaining `status = 'active'` mid-run is
an edge case worth deciding deliberately rather than by accident.

`also_allow` should support the same shape the rest of the config does. Decide
explicitly whether wildcards (`*@sourcehaven.nl`) are in scope; a domain
wildcard is the obvious next request, and it is easier to add deliberately now
than to retrofit around a literal-only matcher.

## Verification

The load-bearing tests are the negatives:

- no `recipients:` block at all → send DENIED, error names the missing config.
- `recipients:` present, address not in the resolved set → DENIED.
- address in the query result → allowed.
- address only in `also_allow` → allowed.
- `allow_any: true` → any address allowed.

Mutation-check the deny-by-default case specifically. A configuration mistake
that silently permits is the failure this ticket exists to prevent, and it is
the one a positive-only test suite would never catch.
