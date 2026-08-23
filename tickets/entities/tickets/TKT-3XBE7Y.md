---
id: TKT-3XBE7Y
type: ticket
title: Clear all doclink findings and promote the rule to a blocking CI gate
kind: enhancement
priority: medium
effort: s
status: in-progress
---

## Problem

`doclink` was adopted as advisory (TKT-MTWQ4G) with 58 findings. Two days later
it was 64. Diffing the finding sets showed **zero new kinds** of broken
reference — the growth was existing breakage propagating as code got copied.
`[Set.Enforce]`, `[Todo.normalized]`, `[Filter]` and `[Hash]` each appeared
twice.

Advisory findings do not get fixed. Clearing the rule once and making it
blocking stops the propagation at its source.

## Why these are real

Go degrades an unresolvable doc link **silently**: `go/doc/comment` keeps it as
plain text, so pkg.go.dev renders the literal characters `[Set.Enforce]`,
brackets and all. `go vet`, `staticcheck` and golangci-lint's `godoclint` all
report zero on a deliberately broken link.

Verified against `go doc` directly that both flagged shapes are genuinely
broken, rather than trusting the tool:

- `[Box.unexportedHelper]` renders with brackets intact — Go cannot link an
unexported member.
- `[Method]` without a receiver renders with brackets; `[Recv.Method]` links.

## The 64 findings and their fixes

| Shape | Count | Fix |
|---|---|---|
| Backticked markdown example (`` `[Title](url)` ``) | 2 | **tool bug** — fixed upstream |
| Bare method missing its receiver | 8 | qualify: `[ForPrincipal] ` → `[Declarative.ForPrincipal] ` |
| Pluralized `[X]s ` | 6 | rephrase so the bracket ends the token |
| `Type.unexportedMember ` | 22 | unbracket — Go cannot link these |
| Method needing a receiver | 10 | qualify |
| Cross-package / test-only symbol | 16 | unbracket — prose naming a collaborator, not a link claim |

Two genuine rename casualties were among them: `[Set.Enforce] ` (the methods
are `EnforceUpdate `/`EnforceCreate `) and `[Declarative.ReadQuery] ` (it is
`Request.ReadQuery `).

## Blocking with an escape hatch

The gate is blocking, and suppression remains available — every rule is a
heuristic over prose, so some findings will be wrong.

That combination has a predictable failure mode: the cheapest path to green is
to silence the finding, and a reviewer skimming a diff cannot tell a considered
suppression from a reflex one. So the failure message (commentlint v0.3.0) leads
with *reading* the finding, presents suppression as the second option, names the
rule in a copy-pasteable directive, and states that "suppressed to unblock CI"
is not an acceptable reason.

## Scope

- 30 files: doc-comment references corrected or unbracketed
- `.commentlint.yml `, `justfile `, `.github/workflows/ci.yml ` — `doclink ` moves
from the advisory loop to the gate; commentlint pinned to v0.3.1

## Upstream changes (separate repo)

- **v0.3.0** — blocking runs print fix-before-suppress guidance
- **v0.3.1** — `doclink ` ignores bracketed text inside backticked spans, which
was a false positive on comments explaining markdown syntax

## Out of scope

The remaining advisory backlog: duplication 120, nil-contract 105,
param-contract 5, restatement 17. Each is its own follow-up.
