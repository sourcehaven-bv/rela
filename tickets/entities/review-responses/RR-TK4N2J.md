---
id: RR-TK4N2J
type: review-response
title: Advisory CI step reports only restatement — the four rules with the real backlog never run
finding: 'The advisory CI step runs `commentlint -rank -top 30` with no -rules flag, which reports only restatement (19 findings). The four opt-in cross-comment rules that carry the actual backlog — duplication 119, nil-contract 100, doclink 58, param-contract 5 — never run, so the backlog the gate/report split exists to surface is invisible in CI. Enabling them in .commentlint.yml does not fix it: each cross-comment rule returns early and replaces the per-comment output, so only one runs per invocation.'
severity: significant
resolution: 'The CI advisory step and the `comment-report` recipe now invoke commentlint once per rule (restatement, param-contract, doclink, nil-contract, duplication), each wrapped in a ::group:: fold in CI. Verified: all five now report, totalling 301 findings and matching the ticket''s table. A comment at both sites records WHY the loop exists — the cross-comment rules replace rather than augment the per-comment output, so a single invocation would silently report only one.'
status: addressed
---

## Finding

The "Comment report (advisory)" step runs:

```
commentlint -rank -top 30 ./internal ./cmd
```

That reports **19 findings** (restatement only), not the 301 the ticket claims
it surfaces. `duplication`, `nil-contract`, `doclink` and `param-contract` are
opt-in (`false` in `DefaultConfig`) and are not enabled in `.commentlint.yml`,
so the step silently omits every rule the ticket was written to surface.

Worse, the advisory step is the ONLY visibility mechanism for the backlog. The
whole gate/report split is justified by "the advisory rules have a real
backlog... they are being worked down" — but nothing in CI ever prints them, so
the backlog is invisible and the justification is hollow.

## Evidence

```
$ commentlint -rank -top 30 ./internal ./cmd
19 findings across 9876 comments (0.2%)
```

Expected per the ticket: duplication 119, nil-contract 100, doclink 58,
param-contract 5, restatement 19.

## Compounding: the rules cannot be combined in one invocation

Enabling them in `.commentlint.yml` does not fix it. Each cross-comment rule
returns early in `main.go` (they have their own output shape), so exactly one
runs per invocation and it REPLACES the per-comment output rather than adding to
it:

```
$ commentlint -config /tmp/test-cl.yml -rank -top 3 ./internal
3 duplicated facts across 8 comment sites   # restatement output is gone
```

So the fix is not a one-line config change — the CI step has to invoke the tool
once per rule.

## Resolution options

1. Run the advisory step once per rule (`for r in duplication nil-contract
doclink param-contract restatement; do commentlint -rules $r ...; done`). Fixes
it here, no upstream change.
2. Fix upstream so cross-comment rules compose in one run, then simplify the
step. Better long-term, but blocks this ticket on a tool release.

Option 1 now; option 2 as a follow-up.
