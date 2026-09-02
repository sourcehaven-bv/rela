---
id: IMPL-KU1WNF
type: implementation-checklist
title: 'Implementation: Attachments on entity create: stage files in the create form, upload after the entity exists'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Frontend only, as planned — **no Go source changed** (`git status` shows only
`frontend/`, `e2e/`, and docs). The hardened upload path is reused verbatim.

| File | Change |
|------|--------|
| `frontend/src/widgets/FileWidget.vue` | staged mode (`canStage`), object-URL preview lifecycle, `stageFile`/`unstageFile`, `acceptFile` branch |
| `frontend/src/widgets/types.ts` | `stagedFiles?: File[]` on `WidgetProps` |
| `frontend/src/components/forms/FieldRenderer.vue` | forwards prop + `update:staged-files` |
| `frontend/src/components/forms/FormFieldList.vue` | forwards prop + `update-staged-files` |
| `frontend/src/components/forms/DynamicForm.vue` | `stagedFiles` ref, dirty term, `uploadStagedFiles`, failure messaging |
| `e2e/tests/fixtures.ts` | `screenshot` (max 1) + `evidence` (max 3) file properties on `bug`, surfaced on its form; `_attachments` on `EntityResponse` |
| `e2e/pages/form.page.ts` | file-control page-object methods |

New tests: `frontend/src/components/forms/DynamicForm.attachments.test.ts` (11),
`e2e/tests/attachments-create.spec.ts` (5), plus 6 staged-mode cases in
`widgets.test.ts`.

**One existing test changed, deliberately.** `widgets.test.ts`'s "shows a note
(no upload control) in edit mode without entity context" pinned the behaviour
this ticket removes — no `entityId` now means CREATE, not "cannot mutate". Its
intent (a genuinely immutable widget shows a note) is preserved by retargeting
it at the `disabled` (policy-read-only) case, which is the real cannot-mutate
condition.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`textFile()` builds test files; assertions compare against the *same* `File`
instance that was staged (`toHaveBeenCalledWith(..., file)`), and the uploaded
id is asserted against the mocked create response rather than a literal.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran a real `rela-server` (fsstore) on a scratch project with `screenshot` (max
1) and `evidence` (max 3), drove the actual SPA in Chromium, and used the API as
the arbiter. Results:

```
1. create-form file control present: YES
2. staged filename shown: manual-shot.png
3. "Pending save" badge: YES
4. uploads before save (must be 0): 0
5. image preview from object URL: YES
6. evidence staged count: 2
7. capacity counter: 2 / 3
8. landed on: /entity/bug/BUG-001
10. screenshot attached: ["manual-shot.png"]
11. evidence attached: ["manual-shot.png","manual-b.txt"]
12. screenshot property stamped: "attachments/BUG-001/screenshot/manual-shot.png"
13. evidence property stamped: ["attachments/BUG-001/evidence/manual-b.txt",
                               "attachments/BUG-001/evidence/manual-shot.png"]
```

Maps to acceptance criteria: AC1 (1,2,8,10,12), AC3 (6,7,11,13), AC-uploads-use-
server-id (12,13 — paths carry the server's `BUG-001`). AC2/AC4/AC5 are covered
by the automated suites (staged-removal, 413/422/403 failure, ACL parity).

**Duplicate-upload check.** A first pass counted 5 `/_attachments/` requests for
3 files, which looked like double-uploading. Re-ran with one staged file and
logged method+path: exactly `PUT /api/v1/bugs/BUG-002/_attachments/screenshot`,
one request. The extra hits were the SPA's post-upload entity refetches, which
match the same URL filter — not duplicates.

**Automated results:** frontend 2022/2022 passing (124 files); e2e 270 passed /
8 pre-existing skips / 0 failures; `go test ./internal/dataentry/...
./internal/attachment/...` ok; `npm run typecheck` clean; `npm run lint` 0
errors; `npx tsc --noEmit` + `eslint` clean in e2e (the page-object rule is a
compile-level error, so the new spec is proven to use page objects only); `just
arch-lint` OK.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Patterns:** staged dirtiness mirrors the existing `pendingCardChanges` term in
`checkDirty`; the two-phase create mirrors the `link_as=from` reverse-relation
write already in `handleSubmit`. Deliberately *diverges* from that precedent on
one point — it only `console.warn`s a failure, whereas a dropped attachment is
user-visible data loss and gets a real error message.

**DRY:** staged rendering reuses the existing `formatSize`, `.file-list` /
`.file-item` markup and capacity computeds rather than duplicating them; the
extra state is one `stagedFiles` ref and two small helpers.

**Security:** no new endpoint, blob namespace, or ACL subject. Every upload
still passes `attachmentWritePreflight` (uniform-404 read gate → file/hidden
property check → lock check → `AuthorizeWrite("update")` with an audit row on
deny) *before* the multipart body is parsed. No client-side check is treated as
a control; the 413/422 remain authoritative. Uploads address only the
server-returned id.

**No silent failures:** every upload failure is collected and named in a message
that identifies the file and reason; a failure also keeps the form dirty so the
navigation guard still warns. The `console.log` used while debugging the test
mock was removed (`grep` for DEBUG-HTML returns nothing).
