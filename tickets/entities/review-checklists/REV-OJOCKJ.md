---
id: REV-OJOCKJ
type: review-checklist
title: 'Review: export: per-list render override (lists.<id>.export_render)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full sweep green (`go test ./...` exit 0) after rebase onto develop @ dd0fe649
- [x] Lint clean (`just lint`) — golangci-lint 0 issues; `just arch-lint` OK (no new `lua`/`script` → `dataentry` edge); `just plimsoll` OK (`Runtime` held at max-methods=120, not bumped)
- [x] Coverage maintained (`just coverage-check`) — package floor 50% PASS, total 65% PASS (76.3%)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-DXDTSB, RR-ECUOIT, RR-YI4LGQ)
- [x] Self-reviewed the diff for unrelated changes

**Reviews run:** cranky-code-reviewer + go-architect, in parallel, plus a
4-agent `/simplify` pass (reuse / simplification / efficiency / altitude) before
them.

**Findings addressed:**

- **RR-DXDTSB** (significant, from the reuse pass) — `buildListQuery` parsed
`filter[...]` keys with `TrimPrefix`/`TrimSuffix`, the exact pre-RR-6RF60V
shape: `filter[status][ne]` keyed the script's table on `"status][ne"`. Now
routed through the existing `parseRelationFilterKey`. Pinned by
`TestExport_List_RenderOverride_FilterOperatorKey`, verified by reverting the
fix and watching it fail.
- **RR-ECUOIT** (significant, cranky) — `rela.document.query.q` was omitted when
empty, so `"Search: " .. query.q` worked on a filtered export and hard-errored
on every unfiltered one. Reproduced (`cannot perform concat operation between
string and nil`), then fixed to always set the field.
- **RR-YI4LGQ** (significant, architect) — `listOverrideRenderer`'s godoc claimed
the override "renders exactly the set the on-screen view showed"; rows in fact
arrive as full entity tables including the **body**, which the column table
never shows, and `visibility.Redact` does not redact `Content` yet. Comment
corrected; both it and the `ListRows` seam godoc now point at the body-redaction
TODO.
- **Minor/nit, addressed:** sort grammar duplicated from `applyV1Sorting`
(extracted `parseSortParam`, aliased `ListSortSpec = filter.SortSpec`);
`ListQuery.Rendered` dead + `.Truncated`/`.ListID` redundant (removed, with
`total` clamped up to the row count so an under-reporting caller can't produce
an incoherent view); `resolveEffectiveList` called twice per export; `sort`
table left unfrozen; `validateExportRenderShape` not shared with views;
`registerListDocumentFields` godoc led with the plimsoll cap rather than the
real reason; `runDocumentScript` seam invariant unstated.

**Findings rejected, with reasons:**

- Cranky claimed list renders run with **no timeout**. Verified false —
`newRuntime` initializes `timeout: DefaultTimeout` and `WithTimeout` is only
appended when non-zero, so `Timeout: 0` inherits 30s. Confirmed empirically: an
infinite-loop script terminates at 30.0s. No change made.
- Two agents called the `cfg.Command` guard in `RenderListMarkdown` unreachable
dead code. Unreachable today, but kept as a fail-closed assertion against an
entry-less `{id}` substitution; comment trimmed to say so.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all PASS — see IMPL checklist for the AC → test mapping.

## Documentation (enhancements only)

- [x] Docs checklist completed — see the linked docs-checklist.
