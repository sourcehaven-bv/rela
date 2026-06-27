---
id: TKT-LR5HLB
type: ticket
title: 'Test hygiene batch: pin vacuous tests, git skip guard, context-aware timeout handlers'
kind: test
priority: low
effort: xs
status: done
---

## Problem

Four small residues from the test-quality review:

1. `TestCreate_CascadeNoRecursion` (entitymanager) contained a long comment walking back its own assertion strategy and pinned almost nothing beyond "completes without error".
2. `TestParseDocument_UnclosedFrontmatter` (markdown) accepted *either* error or success — it pinned no behavior at all.
3. `internal/git` tests fail (rather than skip) on machines without a `git` binary.
4. Two timeout tests waste ~2.5s of wall time per run: their `httptest` handlers sleep a fixed duration, and `server.Close` blocks on the in-flight handler long after the client under test has timed out.

## Approach (agreed with reviewer in session)

1. Cascade test: pin the no-Manager-recursion invariant through `countingStore` call counts — single-dispatch shape is exactly 3 creates / 1 update (requirement + cascade checklist + childAuto's Set via the Create→conflict→Update upsert); Manager re-entry would add a conflict-create+update pair (4/2). Counts probed empirically before pinning.
2. Unclosed frontmatter: deterministic parse error (probed) — assert error containing "frontmatter".
3. `requireGit(t)` LookPath guard in both repo-setup helpers → skip, not fail.
4. Handlers select on `r.Context().Done()` alongside the timer — return as soon as the client gives up, so `server.Close` doesn't block.

## Verification

- All five touched packages green under `-race -count=2 -shuffle=on`; lint 0 issues.
- Timeout test packages: lua 0.8s / ai 0.9s for the targeted runs (was ~2s+ extra in Close).
