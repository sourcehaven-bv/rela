---
id: TKT-MTWQ4G
type: ticket
title: 'Adopt commentlint in CI: comment-discipline gate + advisory report'
kind: enhancement
priority: medium
effort: s
status: review
---

## Problem

Comment quality has no automated signal. `/code-review` catches stale and
misleading comments case by case — the tickets tree holds a long tail of
review-responses about exactly that ("Stale doc comment on X", "Comment falsely
claims Y") — but nothing runs over the corpus and nothing catches a regression
before review.

## Approach

Adopt [commentlint](https://github.com/sourcehaven-bv/commentlint) (MIT,
`sourcehaven-bv`) as a CI job, in the same shape as `arch-lint` and `plimsoll`:
pinned version in the justfile, mirrored in `ci.yml`.

Two steps, deliberately split:

- **gate** — `commented-code`. Clean today (0 findings), so a regression fails
the build.
- **report** — advisory (`continue-on-error`), surfacing the rules that have a
real backlog.

A gate that is red on arrival trains people to ignore the job, so a rule is
promoted to the gate only once its backlog reaches zero.

## Rules and their backlog at adoption

| Rule | Findings | Status |
|---|---|---|
| `commented-code` | 0 | gate |
| `param-contract` | 5 | advisory |
| `restatement` | 19 | advisory |
| `doclink` | 58 | advisory |
| `nil-contract` | 100 | advisory |
| `duplication` | 119 | advisory |

`too-long` and `scope-reach` are disabled outright — see `.commentlint.yml` for
the evidence. Length is a poor proxy for "this comment explains the system": the
mode on this corpus is one line over the threshold, and the highest-value
comment in the tree (a verified sandbox-escape limitation on a one-line
function) has the worst doc:body ratio in the repo. `scope-reach` flags shared
acronyms (ACL, DSN, JWT, SSE) as out-of-scope terms.

## Notable

`doclink` catches dead godoc links, which **no other tool reports**. Go degrades
an unresolvable `[Symbol]` silently — pkg.go.dev renders the literal brackets —
and `go vet`, `staticcheck` and golangci-lint's `godoclint` all return zero on a
deliberately broken link. Most findings are a bare `[Method]` where Go requires
`[Recv.Method]`.

## Scope

- `.commentlint.yml` — rule config, disabled-rule rationale, allow-phrases
- `.github/workflows/ci.yml` — "Comment lint" job (gate + advisory report)
- `justfile` — `comment-lint` (added to `just check`), `comment-report [rule]`
- `CLAUDE.md` — rule documentation next to plimsoll/arch-lint
- `tickets/templates/entities/review-checklist.md` — gate checkbox plus
suppression guidance (inline `//commentlint:ignore <rule>  <reason>` preferred;
`.commentlint.yml` for recurring prose; reason required either way)
- Four genuine restatement fixes found while wiring it up, plus one inline
suppression of a false positive

## Out of scope

Working down the advisory backlog. Each rule's backlog is its own follow-up.
