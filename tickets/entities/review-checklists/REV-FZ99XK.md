---
id: REV-FZ99XK
type: review-checklist
title: 'Review: A conflict-marker property name makes fsstore write a file it can never read back'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` 0 issues; `just arch-lint` no warnings; `just comment-lint` clean
across 13854 comments; `just plimsoll` clean. `go build ./...` and
`go build -tags sqlite ./...` both green, and `go vet ./internal/...` clean.
The full pre-commit suite (fmt, lint, tests) passed on the commit itself.

`just coverage-check` reported one failure, `TestAppEditorBundleEmbedded`,
which was **pre-existing and unrelated**: verified by running it on a clean
`develop` tree, where it fails identically. The cause is local, not a repo
state — `internal/dataentry/app_editor_dist/` is gitignored build output, and
this worktree held a stale bundle predating the editor CSS split, so the test
found a `rela-editor.js` with no accompanying `.css`. Rebuilt with
`npx vite build --config vite.editor.config.ts` (the command the test's own
skip message names); the test passes and the pre-commit suite is green. No
production code was touched for this.

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: autonomous scheduled run; self-review recorded below)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Self-review.** Three things were worth a second look:

1. *Quote or reject?* The first instinct was a rule in
   `storeutil.ValidateProperties`, matching BUG-X7ICNM. Wrong direction: that
   bug's value (invalid UTF-8) is not representable in YAML at all, whereas
   this one is — memstore and pgstore store it correctly today. Rejecting
   would make a limitation of one backend's serializer into a rule about what
   an entity may contain, which is exactly what BUG-B1RA3J's godoc argues
   against. Quoting in `KeyNode` is the narrower fix and keeps the backends
   agreeing.

2. *Scope of the predicate.* Only a key that STARTS with the marker is
   quoted. `"x<<<<<<<"` stays plain, and a test pins that, because the
   emitter puts it at a non-zero column anyway and quoting it would reflow
   existing files for nothing. This matches the line-anchoring BUG-WN6D
   already established for the scanner.

3. *An adjacent hole, found while reading.* Both `FormatDocumentOrdered`
   fallbacks called `yaml.Marshal` on the raw map when `keyOrder` was empty,
   bypassing `KeyNode` and `ValueToNode` — so a caller supplying no order got
   none of the BUG-B1RA3J guards either, not just this one. Routed both
   through `marshalOrdered`, whose empty-order path emits the same
   alphabetical output. This is in scope: it is the same defect on the
   sibling code path, and fixing one without the other is how BUG-B1RA3J's
   `valueToNode` duplication happened.

Diff contains only: the `KeyNode` predicate and its helper, the two fallback
routings, the `ValidateIDPrefix` gate, tests for each, one storetest
conformance case, three fuzz seeds, and ticket entities. No unrelated changes.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | fsstore reads back an entity whose property name begins with the marker | PASS | `RoundTripsConflictMarkerPropertyName` in `storetest`, green on fsstore and memstore |
| 2 | the backends agree | PASS | same case runs from the shared conformance kit, so a new backend inherits it |
| 3 | the assertion is on the emitted bytes, not the decoded map | PASS | `TestMarshalOrdered_ConflictMarkerKeyIsNotWrittenAtColumnZero` checks `HasConflictMarkers` over the frontmatter |
| 4 | the fix does not reflow files it need not touch | PASS | `TestMarshalOrdered_InteriorMarkerKeyStaysPlain`; existing `TestMarshalOrdered_KeysRoundTrip` unchanged and green |
| 5 | an `id_prefix` leading with `_` or `-` fails at metamodel load | PASS | `TestValidateIDPrefix`, with `_x-` moved from the valid list to the invalid one |
| 6 | both fuzz targets pass on the committed seeds | PASS | `FuzzGenerateShortID`, `FuzzCloneNestedValues`, and `FuzzPropertyValuesTypeZoo` (sqlite tag) all ok |

**Negative controls.** Both new assertions were confirmed to FAIL with the fix
reverted — the markdown test and the conformance case each go red when
`startsConflictMarker` is removed from `KeyNode`. Without that check a test
asserting a property of correct code proves nothing.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Two notes for a later reader:

- The third target in the sweep (`sqlitestore FuzzPropertyValuesTypeZoo`, a
  `"/"` property name) already passed on `develop`. Its input is committed as
  a regression seed rather than dropped, but no code changed for it.
- The bug entity's own body had to indent its YAML example by one space:
  written flush-left it made THIS file scan as conflicted, which
  under-counted `rela analyze` until it was fixed. The defect demonstrating
  itself inside its own ticket is the clearest argument that the scan runs
  where the round-trip oracle cannot see.
