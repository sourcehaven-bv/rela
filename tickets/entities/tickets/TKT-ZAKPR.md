---
id: TKT-ZAKPR
type: ticket
title: Pre-push hook runs arch-lint, build, lint locally
kind: chore
priority: medium
effort: xs
status: done
---

## Problem

Bugs BUG-0F0K (arch-lint config drift) and BUG-7OMLX (scheduler used unexported
`workspace.meta()` after squash merge — hidden compile error after rebase) both
reached develop because no local check ran them before push. CI catches both but
the round-trip is slow and pollutes the timeline.

The user's general direction is "checking stuff locally is preferred over
waiting on CI". The existing `scripts/pre-push` hook already enforces ticket
presence — extend it with the cheap, deterministic checks.

## Scope

**In scope**

- Augment `scripts/pre-push` to additionally run `just arch-lint`,
`just build`, and `just lint` when Go-relevant files have changed.
- Skip the Go checks if no Go files / `go.mod` / `go.sum` / `.go-arch-lint.yml`
have changed in the push range — keeps doc-only pushes fast.
- Print a clear failure message with the failing recipe and a hint that the
user can rerun locally.

**Out of scope**

- Running `just test` in pre-push (too slow for the user's "cheap local
check" bar).
- Running e2e / Playwright in pre-push.
- Adding `just coverage-check` (slow + already CI-enforced).

## Acceptance criteria

- Push of a doc-only change still skips Go checks (fast).
- Push that touches `*.go` or `go.mod` runs `just arch-lint && just build &&
just lint` and aborts on failure.
- Hook runs once per push, not once per ref.
- Existing ticket-presence check remains intact.
