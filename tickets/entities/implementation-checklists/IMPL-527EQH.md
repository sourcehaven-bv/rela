---
id: IMPL-527EQH
type: implementation-checklist
title: 'Implementation: Extract typeResolver + trace/export handlers off mcp.Server (plimsoll ratchet 49 → 38)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new behavior — receiver-only moves; dispatch/golden suites pin the surfaces)
- [x] ~~Integration tests written~~ (N/A: same)
- [x] Happy path implemented
- [x] Edge cases from planning handled (toolGetSchema/toolGetMetamodel aliasing untouched; principalMiddleware left at SDK level)
- [x] Error handling in place

## Test Quality

- [x] ~~Fixture builders~~ (N/A: test changes are mechanical re-points)
- [x] ~~No hardcoded values in assertions~~ (N/A)
- [x] ~~Only values that matter~~ (N/A)
- [x] ~~Interpolated values from objects~~ (N/A)
- [x] ~~Property comparisons from original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end (full suite + `-race ./internal/mcp/...`)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence (PR #1463, commit b013fb44):**
- Server 49 → **38**; reviewer independently counted 38 and confirmed the
directive has no slack.
- `typeResolver` (meta only), `traceHandler` (GraphReader + tracer),
`exportHandler` (GraphReader only — one dep, the cleanest cut).
- Reviewer's normalized-receiver diff showed the moved bodies are
byte-identical modulo field renames: no statement, branch, error string or
argument order changed.
- Doc-drift fix verified accurate: `mcp.Services` is genuinely gone from
code; remaining mentions are archived tickets, correctly untouched.
- Gates: build, go vet, golangci-lint (0 issues), arch-lint, comment-lint
(11071 comments, no unresolvable doclinks), `-race` — all green.

## Quality

- [x] Code follows project patterns (urlHelpers precedent — reviewer verified it's a real in-repo precedent, not an invented justification)
- [x] Checked for DRY opportunities (one shared `Deps.handlers` derivation so production and test wiring can't drift)
- [x] No security issues introduced (handlers hold the narrow GraphReader, never store.Store, so a networked wiring substitutes a visibility-gated reader without touching them; no principal field on any handler — threading identity in would now require adding a visible field)
- [x] No silent failures
- [x] No debug code left behind
