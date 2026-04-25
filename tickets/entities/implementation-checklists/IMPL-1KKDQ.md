---
id: IMPL-1KKDQ
type: implementation-checklist
title: 'Implementation: Pre-push hook runs arch-lint, build, lint locally'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: bash hook script; smoke-tested
      end-to-end below)
- [x] ~~Integration tests written~~ (N/A: bash hook script)
- [x] Happy path implemented (Go checks gated on file extensions)
- [x] Edge cases handled (deleted branches, new branches without remote ref,
      doc-only pushes, ticket-only pushes)
- [x] Error handling in place (each Go check aborts with a hint message)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: shell script)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `bash -n scripts/pre-push` syntax check passes.
- Hook executed against `origin/develop~1..origin/develop` (a Go-touching
  range): runs `just arch-lint` (no warnings), `just build` (built
  successfully), `just lint` (0 issues), prints `pre-push: All checks
  passed!`. Total runtime ~3 minutes.
- Gating regex tested in isolation against four input cases: `CLAUDE.md`,
  `internal/foo.go`, `go.mod`, `tickets/entities/tickets/TKT-X.md`. The Go
  filter triggers only on `.go`, `go.mod`, `go.sum`, `.go-arch-lint.yml`,
  `.golangci.yml` — doc and ticket changes correctly skip the heavy checks.
- `just install-hooks` re-installs the updated script.

## Quality

- [x] Code follows project patterns (matches existing hook style)
- [x] No security issues introduced
- [x] No silent failures (each `just` recipe is checked with an explicit
      `if !` and aborts on non-zero exit)
- [x] No debug code left behind
