---
id: PLAN-7BRRNU
type: planning-checklist
title: 'Planning: Ship a generated operator handbook for the demo tracker (dogfood rela-docs)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented

**Scope:** IN: commit a generated operator handbook
(`docs/examples/ticket-tracker-manual.md` + screenshot) built by `rela-docs`
from the demo project; a `just docs-example` target; a README link; fix the
double-blank seam in the docs renderer. OUT: making the README itself a manual
(rejected — manual memstore can't read rela's real on-disk guide entities, the
README's dynamic tables).

**Acceptance:** see ticket. All met (verified below).

## Research

- [x] Built on the existing `prototypes/data-entry/manual/tickets-manual.md` (already authored earlier in the arc) rather than starting fresh.
- [x] Chose the demo project (`prototypes/data-entry/project`) because it carries a showcase `acl.yaml` (editor/viewer) → real roles_matrix, and the screenshot pipeline already targets it.

## Approach

- [x] Technical approach documented
- [x] Builds on existing patterns

**Approach:** Polish the existing manual to English; build it to
`docs/examples/` via `rela-docs build`; commit the `.md` + PNG. Add a `just
docs-example` target kept OUT of `docs`/`docs-check` (screenshot PNG isn't
byte-reproducible, so committed output is the source of truth). Link from the
README via `generate-docs.lua` (README is Lua-generated). Fix the MD012
double-blank at island seams in `internal/docs` (`normalizeBlankLines`: collapse
2+ blank lines, trim trailing).

**Files:** prototypes/data-entry/manual/tickets-manual.md, docs/examples/*
(new), justfile, scripts/generate-docs.lua, README.md (regen),
internal/docs/runtime.go + build_test.go.

## Security Considerations

- [x] No new inputs; a docs artifact. The screenshot stands up an ephemeral seeded memstore/fsstore — no real data.

## Test Plan

- [x] Test scenarios documented
- [x] Edge cases identified

**Tests:** `normalizeBlankLines` unit table (double/triple/trailing/spaces) +
`TestBuild_NoDoubleBlankAtSeam` (no `\n\n\n` in output). Markdown lint on the
generated handbook + README (0 issues). Manual render verified end-to-end incl.
the annotated screenshot.

## Risk Assessment

- [x] Risks assessed; effort m

**Risks:** `normalizeBlankLines` could collapse intentional blank runs inside a
rendered code fence — low risk (resolver-emitted fences contain no blank runs;
author code samples rarely do, and MD012 would flag them anyway). Flagged for
review. Screenshot non-determinism → kept out of docs-check.

## Documentation Planning

- [x] This IS documentation. README links the new example.

## Design Review

- [x] ~~/design-review~~ (N/A: small; the one non-trivial bit — the renderer normalization — gets a focused code review in the review phase).
