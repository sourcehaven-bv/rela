---
id: TKT-USQNA3
type: ticket
title: Operator-configured recipient allowlist for mail.send
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

`mail.send`'s `to` field is entirely script-chosen with no operator constraint.
Even with the capability gate (TKT-JVHSOZ), a script that legitimately holds the
`mail` grant can address any recipient it likes.

## Scope — RESCOPED

Originally specified as a graph query (`person where status = 'active'`) with a
literal fallback. **Rescoped by the project owner to domains and literals
first**, deferring the query form.

Two reasons, and the second is the one that actually decided it:

**1. The query form has an unresolved architectural question.** Resolving
`person where status = 'active'` needs `filter`, and `.go-arch-lint.yml`
withholds it from `internal/mail` deliberately — the dependency comment there
says a send script has no graph access "by construction rather than by
convention". That is a boundary worth keeping, and working around it is a design
decision rather than an import fix.

**2. Domains deliver most of the value at a fraction of the cost.** The threat
is a script mailing an attacker-chosen address. `*@sourcehaven.nl` stops that
without needing to know which people currently exist. The query form's advantage
— tracking the graph as people join and leave — matters for WHO inside the org
receives mail, not for whether mail leaves the org at all.

IN:

```yaml
recipients:
  also_allow:
    - "*@sourcehaven.nl"     # domain pattern
    - "ops@example.com"      # literal
  # or
  allow_any: true
```

- **Absent means DENY ALL.** Deliberately inverting this file's usual rule (an
absent `mail.yaml` means mail is off; an absent port means 587). Permitting on
absence fails silently and irreversibly — mail leaves the ACL perimeter and
nobody knows until the recipient replies. Refusing on absence fails loudly and
harmlessly: a typed error naming the key, and four lines of YAML to fix it. A
control whose unconfigured state is "allow" is not a control.
- `allow_any: true` is the explicit escape hatch. Never a default, never
inferred from an empty block, never reached by omission — so it stays greppable
in a config review.

OUT, deferred to a follow-up:

- The `query:` / `property:` graph form. The architecture question above needs
answering first: either resolve the query outside `mail` and pass the address
set in, or widen the arch-lint boundary with a stated reason.

OUT: backwards compatibility, waived by the project owner, consistent with
TKT-JVHSOZ.

## Design note carried forward

The parse/enforce split the earlier draft established is CORRECT and stays:
`internal/mail` parses and validates the block; `internal/lua` enforces it,
because that is where the send actually happens. When the query form lands it
must not disturb that — resolution needs graph access, and the whole point of
the split is that `mail` does not have it.

## Verification

The load-bearing tests are the NEGATIVES, and deny-by-default most of all: a
config mistake that silently permits is the exact failure this prevents, and a
positive-only suite would never catch it.

- no `recipients:` block → DENIED, error names the missing config
- address matching no pattern → DENIED
- literal match → allowed
- domain-pattern match → allowed
- `allow_any: true` → any address allowed
- a pattern that is not a leading `*@` → refused at LOAD, not silently treated
as a literal
