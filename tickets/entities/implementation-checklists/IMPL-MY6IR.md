---
id: IMPL-MY6IR
type: implementation-checklist
title: 'Implementation: Relocate .rela/ user-local state to user config directory (cross-platform)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test -race ./... ` — all green.
- `just lint` — 0 issues.
- `~/go/bin/go-arch-lint check` — OK, no warnings.
- `go test ./internal/userstate/...` — coverage 80.3%.
- `go test ./internal/project/...` — coverage 85.2%.

Manual spot-checks:
- `NewForTest` round-trip Get/Put in userstate/fs_test.go (AC1, AC4)
- Identity precedence env > userstate via loader_test.go (AC2)
- `NewLocalState(svc)` + lockedfile concurrent writers (AC3)
- Factory constructor rejects nil UserState (AC6)
- Cross-platform path resolution `resolveBase` table tests (AC7, AC8)
- `$RELA_USER_STATE_DIR` override inside projectRoot rejected (AC9)
- `.rela/repo-id` git-tracked check surfaces ErrRepoIDTracked (AC10)
- `StateFilePerm = 0o600` enforced in userstate/fs.go (AC11)
- `tagNotIndexed` in userstate/platform_{darwin,windows}.go (AC12)
- Error strings audited in app/factory.go and internal/cli/keys.go (AC13)
- Production paths (workspace.Discover, cmd/rela-desktop) now verify keyring repo-id (RR-1L4QP)

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
