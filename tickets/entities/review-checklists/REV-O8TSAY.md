---
id: REV-O8TSAY
type: review-checklist
title: 'Review: Unified targeted-write primitive: entitymanager.PatchEntity replaces four hand-rolled property merges'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./...` 0 failures · `just lint` 0 issues · `just arch-lint` OK · `just
lint-md` 0 issues · `just coverage-check` PASS (77.0%, up from the 76.9%
baseline) · `go test -race` clean on `lua`, `cli`, `entitymanager`.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7YXR83, RR-JZBP5E, RR-MU0TLH, RR-OFWA2Q, RR-E812RW,
RR-TEA3NA

| ID | Sev | Status |
|---|---|---|
| RR-7YXR83 | significant | addressed — structural not-found detection |
| RR-JZBP5E | significant | addressed — nil-map invariant pinned |
| RR-MU0TLH | minor | addressed — inert-gate disclosure in godoc |
| RR-OFWA2Q | minor | addressed — CLI body-flag handling |
| RR-E812RW | nit | addressed — corrected overstated godoc |
| RR-TEA3NA | minor | **deferred** with reasoning (MCP pre-read) |

Zero critical. No open critical/significant.

**The one real defect was mine.** `luaUpdateEntity` detected a missing entity
via `strings.Contains(err.Error(), "entity not found")`. Several hard errors
interpolate caller-supplied values — `statemachine` formats an illegal
transition as `%s %q→%q is not a declared transition` — so a script setting a
property to the literal `"entity not found"` had its transition rejection
reported as a 404, and a script branching on that message could try to recreate
a row that still exists. The reviewer confirmed with a runnable repro against
the real manager.

Fixed structurally: `entitymanager` returns an error exposing `EntityNotFound()
bool`; `lua` matches it through a consumer-side `NotFoundError` interface with
`errors.As`. `errors.Is(err, ErrEntityNotFound)` still works via `Unwrap`, so
the 14 existing sentinel users are unaffected.

Two candidate fixes were rejected after checking rather than assuming: moving
the sentinel into `internal/entity` (14 files across many packages), and
duplicating the sentinel in `lua` — I verified two separately-constructed
`errors.New` values with identical text do **not** match under `errors.Is`, so
that would have silently never fired.

**Regression test verified against the old implementation:**
`TestUpdateEntity_NonNotFoundErrorIsNotMisreported` FAILS with the string match,
PASSES with the structural check.

**Self-review:** every touched file traces to the ticket. The 30-odd peripheral
test diffs are one-line `FieldGate:` additions and `WritePrepStore:` removals
(plus gofmt realignment); confirmed by diffing `analysis`, `attachment`, `docs`,
`validation` for stray edits. No scope creep.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all 11 ACs plus the RR-32XA5V ordering test PASS — full
table with per-AC evidence in IMPL-T8S8LU. Highlights:

- **AC2 (the inverse test)** PASS — `TestPatchEntity_PreservesUnnamedProperties`.
The test that would have caught this bug class at its origin.
- **AC5 (success criterion)** PASS — `WritePrepStore` deleted; 0 references,
`luaUpdateEntity` holds 0 store references.
- **AC1** PASS — and the audit half was **mutation-tested**: disabling
`recordEntityAudit` makes it fail, restoring it passes. Necessary because
`entitymanager/CLAUDE.md` warns audit is inherited by method *name*, and
`PatchEntity` is not in that set.
- **AC7** PASS — verified against the real `cmd/rela` binary on an
fs-backed project, not a harness.

## Documentation (enhancements only)

Internal refactor with one user-visible CLI capability change, so docs were
updated inline rather than via a separate docs-checklist:

- [x] `CLAUDE.md` — the defensive "never redact a read that feeds a write"
prose replaced by the positive `PatchEntity` rule. This was the point of the
ticket.
- [x] `internal/entitymanager/CLAUDE.md` — which-write-method table plus the
load-bearing `PatchEntity` ordering rationale.
- [x] `docs/cli-reference.md` **and its `docs-project/` source** — `-U/--unset`,
`--clear-body`, the `-P key=` vs `-U` distinction, `-B` empty-file semantics.
Also filled in `-P`/`-b`/`-B`, previously undocumented.

**Docs Checklist:** N/A — refactor; the user-facing surface is the three CLI
flags, documented above. `just lint-md` clean; generated file and source
verified byte-identical in the changed section.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Six commits, each explaining the reasoning and citing the review-response it
answers. Deliberately sequenced so the risky `updateCore` extraction is a
standalone behaviour-preserving commit reviewable on its own.

**Known limitation, disclosed not hidden:** `FieldWriteGate` is wired as
`AllowAllFieldGate` at every production site, so it is inert outside tests. This
preserves today's exact behaviour (nothing outside dataentry ever field-gated
writes) and the seam makes the real wiring a config change rather than a
rewrite. Stated in the `FieldWriteGate` godoc itself — not only in TKT-0XL8MF —
so nobody builds on protection that is not yet there.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

All 21 code checks green: Test (`-race -shuffle=on`), Lint, E2E, Postgres
Backend, Architecture, Frontend, Build, God-object lint, Lint Markdown,
Vulnerability Check, Fuzz, CodeQL (go / actions / js-ts), and all 7
Cross-Compile targets including both postgres variants.

The `Rela Tickets` check failed while this box was unticked — correctly. It
enforces "done review checklists cannot have unchecked items", which cannot
be satisfied until CI has actually run. Resolved by finishing the workflow
(tick once green, move the ticket to `done`), not by weakening the rule.

`Docs` regenerates from `docs-project/` and fails on any diff. Verified
locally by running `./scripts/generate-docs.sh` — the generator reproduces
the cli-reference edit byte-for-byte, because the source entity was edited
alongside the generated file.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1306
