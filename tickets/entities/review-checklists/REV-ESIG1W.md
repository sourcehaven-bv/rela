---
id: REV-ESIG1W
type: review-checklist
title: 'Review: export: documents export via transforms, like entity and list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` — `internal/dataentry` and `internal/dataentryconfig` pass.
Full `go test ./...` passes except `TestMemoryLocker_AbandonedAcquireReleases`
(`internal/lock`), which is **pre-existing and unrelated**: reproduced on a
clean `develop` worktree, and this branch does not touch that package.
- [x] `just lint` — **0 issues**.
- [x] `just arch-lint` — **OK - No warnings found**.
- [x] `just coverage-check` — thresholds met (`internal/dataentry` 82.4% vs floor
55; `internal/dataentryconfig` 91.2% vs floor 70). The recipe's only failure is
the same `internal/lock` flake, which surfaces under its `-race -shuffle=on`.
- [x] `vue-tsc` clean; `eslint` 0 errors on changed files; frontend suite passes.

## Code Review

Two reviewers ran in parallel against commit `d0e4b7d0`.

**Security review: no critical or significant findings.** It verified all five
design findings were properly addressed, and independently confirmed the gate
ordering, the flat-500 script-error choice, that `GetCached` is unreachable from
export, that `RenderStandaloneMarkdown` kept `elevatedDeps`, the hardened
download headers, and that the transform engine is genuinely shared. It also
noted the `lint_test.go` allowlist was **narrowed** rather than widened, turning
"every caller checked the gate" from a policed convention into a structural
property.

**Code review: 1 significant, 5 minor/nit.** Findings and disposition:

| Finding | RR | Disposition |
|---|---|---|
| Empty interior path segment served a standalone doc at the anchored shape | RR-1JZTB0 | **Fixed** |
| RR-T5POTJ promised an elevation test that was never written | RR-T5POTJ | **Fixed** (test written) |
| Three wrong/misplaced comments in `validate.go` | RR-R1XKH4 | **Fixed** |
| `quoteName` drops `%q` escaping | — | **Fixed** (replaced with `fmt.Sprintf`/`%q`) |
| Empty-segment loop could use `slices.Contains` | — | **Fixed** |
| `entryID == ""` sentinel encodes document kind | RR-S8P0GQ | Deferred, recorded |
| Frontend URL-encoding inconsistency (pre-existing) | RR-S8P0GQ | Deferred, recorded |
| `exportFilename` picks `.conf` for `text/plain` (pre-existing) | RR-JW2XT7 | Deferred, recorded |

Both reviewers independently found the empty-segment routing bug, which is the
strongest signal it was real. I reproduced it by execution before fixing:
`/_documents/sales//_export` returned 200 and rendered the standalone document
at the anchored URL shape — exactly what this package's CLAUDE.md forbids.

## Verification of Acceptance Criteria

| AC | Status | Evidence |
|---|---|---|
| 1 anchored export | PASS | `TestExportDocument_Anchored`; manual e2e returned the rendered body + `ticket_report-TKT-001` attachment |
| 2 standalone export, `entry_id` nil | PASS | `TestExportDocument_Standalone`; manual e2e with a **real Lua runtime** printed `entry_id is nil` |
| 3 every gate enforced | PASS | `TestExportDocument_Gates`, `_PermissionGate`, `_ElevatedGateAppliesToExport` (5 ACL impls); each asserts the renderer never ran |
| 4 transform 400/404 | PASS | `TestExportDocument_TransformResolution` (4 cases) + manual e2e |
| 5 `command:` refused | PASS | `TestExportDocument_CommandRendererRefused` + `TestRenderDocumentMarkdown_RefusesCommandRenderer` backstop |
| 6 Export menu shows/hides | PASS | `ExportMenu.test.ts` (4 cases) |
| 7 document named `_export` rejected | PASS | `TestValidateDocuments_ReservedExportNameRejected` |
| 8 hidden ≡ absent 404 | PASS | `TestExportDocument_NoExistenceOracle` on **both** routes, `maps.Equal` on the whole problem doc |
| 9 script error leaks nothing | PASS | `TestExportDocument_ScriptErrorCarriesNoDetail` (4 leak vectors incl. `CapturedOutput`) |

## Review Summary

Four tests in this change were **verified to fail without their fix**, which is
the bar I held rather than "the test passes":

1. `TestExportDocument_NoExistenceOracle` — reinstated the pre-fix ordering,
confirmed failure on both the render and export subtests.
2. `TestRenderDocumentMarkdown_RefusesCommandRenderer` — removed the backstop,
confirmed the `anchored` subtest fails.
3. `TestExportDocument_ElevationAndCapabilitiesReachTheRender` — swapped
`elevatedDeps` for `luaDeps`, confirmed both assertions fail.
4. `TestExportDocument_RouteShapes` — the empty-segment cases failed against the
committed code before the router fix.

The most useful outcome of this review was not the export feature but RR-XIYP3I:
a pre-existing entity-existence oracle on the document render route, found
during design review and closed here because this ticket extracted that exact
gate chain. Shipping it onto a second route would have doubled it.

One process lesson recorded in RR-R1XKH4: three wrong statements in one small
comment region, all because that block was written from the plan before the
dispatch existed and never re-read against what executes. Neither `go vet` nor
the `doclink` gate can catch a comment that is merely false.
