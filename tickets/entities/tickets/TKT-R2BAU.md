---
id: TKT-R2BAU
type: ticket
title: Custom analyzer or /code-review checklist for silent default returns
kind: test
priority: low
effort: m
status: backlog
---

## Problem

Cross-cutting analysis of 659 review-responses found ~91 findings tagged
"silently swallows", "fails open", "silent semantic conversion" — the single
largest category of significant/critical findings. Patterns:

- `return "", nil` or `return nil, nil` in `default:` and `case error:`
branches with no prior log call.
- Type-mismatch fallthrough to lexicographic compare or `[foo bar]` rendering.
- Cleanup goroutines that swallow `Walk` errors.

Two complementary mitigations are possible; this ticket scopes both for
discussion before committing to either.

**Option A — custom golangci-lint analyzer.** Pattern-match `return ""` /
`return nil` from default/error branches with no preceding `slog.*` call in the
same function. Risk: high false-positive rate; analyzer maintenance.

**Option B — `/code-review` checklist line.** Add a bullet: "Grep the diff for
default returns on error/unknown branches. Each must log+propagate or carry an
explicit `// intentional fallback: <reason>` comment." Lower ceiling, but no
false-positive cost and immediate effect.

## Scope

**In scope**

- Decide between A, B, or both.
- If B: edit `.claude/commands/code-review.md` and the cranky-code-reviewer
agent prompt to include the new check.
- If A: spike a ruleguard or custom analyzer; benchmark FP rate on
develop's diff history; decide whether to enable.

**Out of scope**

- Bulk-fixing existing silent-default sites (the 91 already addressed are
the corpus that motivated this).

## Acceptance criteria

- Either an analyzer is enabled with measured FP rate on a sample, or the
/code-review checklist explicitly asks reviewers to grep for silent defaults.
