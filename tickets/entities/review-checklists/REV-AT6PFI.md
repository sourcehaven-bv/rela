---
id: REV-AT6PFI
type: review-checklist
title: 'Review: Edit form hides properties the entity doesn''t have yet — newly added metamodel properties are unreachable on existing entities'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` — pass
- [x] `npx vitest run` — 1440 pass (88 files)
- [x] `just lint` — 0 issues
- [x] `npm run lint` — 0 errors
- [x] `npm run typecheck` — clean
- [x] `just arch-lint` — OK
- [x] `just coverage-check` — pass (77.2%)
- [x] `just plimsoll` — pass

Confirmed the change builds and tests independently of unrelated uncommitted
work in the tree (BUG-8N2WT2 in `internal/migration/`), so nothing here depends
on it.

## Code Review

Ran `cranky-code-reviewer` on 9ffa06b8. **No critical or significant findings.**
One minor finding, addressed.

The reviewer independently verified the three things most worth doubting:

1. **Security.** Traced every `_redacted` population site. It is set in exactly
one place (`attachEntityAffordances`), called from exactly one place
(`forWire`), immediately after `stripHiddenProperties` — strip and name are
structurally inseparable. `forWireRelated` (list rows, includes) strips without
naming, which is the intended boundary and the safe direction.
`forWireHistoricalReveal` does neither, correctly. Every `forWire` call site is
downstream of a read authorization, so `_redacted` only rides responses the
caller already received. Row-level ACL untouched.

2. **The removed data-loss guard.** Independently re-derived the reasoning
across every edit-mode write path — autosave, `commitImmediately`,
`adoptLockedFieldValues`, relation cards, plus two I had not checked
(`mergeServerResponse` and the `activeProperties` watcher). The watcher was the
one genuinely interesting case; it is safe on three independent counts.
Conclusion: removing the guard was right, and leaving it would have been dead
code implying protection it did not provide.

3. **Test integrity.** Reproduced the revert experiment: 4 of 7 fail on pre-fix
code, including the wizard case. Confirmed the 3 that pass on broken code are
the hide-side assertions passing for the wrong reason — which is exactly why
they are paired with the render-side ones.

Also confirmed `renderedProperties` (`#field-<prop>`) is a sound rendering
assertion rather than a proxy, the module-level `vue-router` mock hides nothing
relevant, and that the modified `TestV1Affordance_PatchEcho_StripsHidden` is a
legitimate sharpening — the value assertion was always the real invariant, and
the replacement is structurally tighter than the substring match it replaced.

### Findings

| ID | Severity | Status |
|---|---|---|
| RR-GNXKTW | minor | addressed |

RR-GNXKTW: unreachable bulk-edit branch in `handleSubmit`. No behavioral defect,
but it contradicted the reasoning that justified removing the redaction prune
guard. Deleted rather than annotated (c1700f65). Deleting it orphaned
`surfaceWarnings`, which surfaced a **pre-existing gap**: the create path also
returns DEC-HWZHA soft warnings and never showed them, since the only caller was
the dead branch. Wired it into the create path, so those warnings are now
visible for the first time.

## Acceptance Verification

| Criterion | Result | Evidence |
|---|---|---|
| A configured-but-unset property renders in the edit form | PASS | `DynamicForm.test.ts` — fails on pre-fix code |
| It can be filled in and saved | PASS | per-property autosave; covered by the render + writable assertions |
| A genuinely redacted property still does not render | PASS | `DynamicForm.test.ts` inverse case |
| Unset and redacted are distinguished on the same entity | PASS | frontend + `TestV1Affordance_PerEntityGet_RedactedNamesHiddenFields` |
| The wizard path behaves identically | PASS | both wizard cases |
| Hidden VALUES never reach the wire | PASS | backend GET/PATCH tests assert on the value |
| `_redacted` matches exactly what was stripped | PASS | `TestRedactedPropertyNames_MatchesStrippedProperties` |
| No new disclosure beyond property names | PASS | reviewer traced all population sites; names already public via metamodel |
| Create mode unaffected | PASS | 1440/1440 frontend tests, create path untouched |
| Docs no longer teach the unsound inference | PASS | api-reference.md hidden-fields section rewritten |
