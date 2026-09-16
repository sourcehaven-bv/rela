---
id: IMPL-N7QF2M
type: implementation-checklist
title: 'Implementation: Deleting an entity panics when comments are disabled (typed-nil subscriber in alias fanout)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — three tests in `internal/appbuild/comments_test.go` covering the typed-nil filter, the mixed dead/live case, and the real `buildComments` output
- [x] ~~Integration tests written~~ (N/A: the defect is in wiring, and the wiring function is reachable directly — an end-to-end delete would exercise the same `newAliasFanout` call through far more setup without testing anything the unit tests miss)
- [x] Happy path implemented — `newAliasFanout` filters through `isNilSubscriber`, so a disabled subsystem leaves the hook unwired and the Manager's nil fast path applies again
- [x] Edge cases handled — `isNilSubscriber` checks `Kind` before `IsNil`, so a non-pointer subscriber answers false instead of panicking inside the guard itself; a live subscriber beside a dead one is still unwrapped rather than fanned out
- [x] Error handling in place — the fix removes a panic; no new error paths. Subscriber errors still join rather than short-circuit, unchanged

## Test Quality

- [x] Using fixture builders — reuses the file's existing `recordingRewriter`, `paths` and `commentsMeta` helpers
- [x] No hardcoded values in assertions — asserts on the recorded notifications, not on literals invented by the test
- [x] Only specifying values that matter — `TestAliasFanout_DisabledCommentsSurviveDelete` asserts the hook is nil and nothing else about the service
- [x] Interpolated values constructed from objects — the ids asserted (`TKT-1`, `TKT-old->TKT-new`) are the ones passed in, recorded by the subscriber
- [x] Property comparisons use original object — `require.Same(t, live, got)` pins identity, not a copy

## Manual Verification

- [x] Feature manually tested end-to-end — reverted the fix to the original `s != nil` filter and re-ran the new tests: both `TestAliasFanout_SkipsTypedNilBesideLiveSubscriber` and `TestAliasFanout_DisabledCommentsSurviveDelete` fail with the production error, `invalid memory address or nil pointer dereference`. Restored the fix and they pass
- [x] Each acceptance criterion verified — a delete with comments disabled no longer panics; a rename on the same path is covered by the same filter
- [x] Edge cases verified — the pre-existing literal-`nil` tests still pass, so the stricter filter did not change the untyped-nil behaviour they pin

**Verification Evidence:** Two live panics on atlas, 2026-09-16 06:57:30 and
06:57:32 UTC, both `comments.(*Service).EntityDeleted` at `service.go:185` via
`aliasfanout.go:68`. Commenting is off on that host, which is the disabled path.
Traced the boxing to `appbuild.go:1763` passing `buildComments`' nil
`*comments.Service` into the variadic. Confirmed the same shape exists a second
time at `appbuild.go:2182` for `*caldavalias.Service`, which is why the fix went
into `newAliasFanout` rather than the call site. `go test` green for
`internal/appbuild/...`, `internal/comments/...` and `internal/entitymanager/...`;
`golangci-lint run internal/appbuild/...` reports 0 issues.

## Quality

- [x] Code follows project patterns — the doc comment explains the reasoning in the style the surrounding comments use, and names the two concrete typed-nil sources
- [x] Checked for DRY opportunities — considered a type switch over the two known subscribers and rejected it: the defect is the wiring pattern, so a future optional subsystem would reintroduce the crash while the switch stayed silently green
- [x] No security issues introduced — a delete that previously panicked now completes; no change to authorization, and no new input is trusted
- [x] No silent failures — a dead subscriber is dropped deliberately, which is the documented contract; a live subscriber's errors are still joined and surfaced
- [x] No debug code left behind
