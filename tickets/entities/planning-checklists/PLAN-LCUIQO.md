---
id: PLAN-LCUIQO
type: planning-checklist
title: 'Planning: Attachments on entity create: stage files in the create form, upload after the entity exists'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN — frontend only (`frontend/src/`):
1. `FileWidget` gains a **staged mode**: with no real `entityId`, hold selected
`File` objects in local state, render them (name, size, image preview via object
URL, remove), upload nothing.
2. `DynamicForm` collects staged files per property and, **after** a successful
create, uploads each against the returned id via the existing
`uploadAttachment`, then refreshes.
3. Upload failure after create is surfaced, never silent: the user lands on the
created entity with the server's problem+json detail inline.
4. Respect the property's `max` while staging (parity with edit mode).

OUT — no Go changes at all. No new endpoint, no new blob namespace, no new ACL
subject. Also out: `required: true` file properties (→ TKT-87VSDE), atomic
multipart create, a staging/unowned-blob namespace, API/MCP create-time attach,
and the pre-existing stale-file-property-after-rename issue. All recorded on the
ticket with rationale.

**Acceptance Criteria:**

1. **Create with a file, one gesture.** Create form shows a working file control;
pick a file, Save; the entity exists with the file attached and the property
stamped. → e2e `attachments-create.spec.ts`.
2. **Staged file removable pre-save.** Stage, remove, Save → no upload issued.
→ unit (FileWidget) + form-level unit asserting `uploadAttachment` not called.
3. **`max > 1` stages up to max.** N files stage and all upload; add control
disappears at capacity. → unit (widget), mirroring the existing "hides the add
control at capacity" test at `widgets.test.ts:583`.
4. **Post-create upload failure is loud.** Oversize/rejected file → entity still
reachable, user lands on it, server detail shown inline, no silent loss. → unit
with a rejecting `uploadAttachment` mock.
5. **ACL parity.** No `update` on the created entity → same 403 the edit path
gives; staged UI does not claim success. → unit (403 mock) + existing Go `acl_*`
coverage already pins the server side.
6. **Uploads address only the server-returned id.** No client-side placeholder
reaches a request URL. → unit asserting the upload mock's args. (Corrected per
RR-QTCKCW: the `++new++` STAGED_ID sentinel has NO production consumer — create
mode passes `entityId === undefined`. The original criterion described a
mechanism that does not exist.)
7. **No regression for `required` file props.** Behaviour identical to today.

## Research

- [x] For larger features: run `/research` — **N/A**, approach settled in the
investigation that produced this ticket; the constraints are mechanical, not
open-ended.
- [x] Searched for existing libraries — **N/A**, no new dependency. Staging is
an array of native `File` objects; `axios` multipart upload already exists.
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — see ticket body, which carries the full option analysis
(A / multipart-create / staging namespace) and why A wins.

**Existing Solutions:**

- **The whole server side already exists and is reused verbatim.**
`uploadAttachment` (`frontend/src/api/attachments.ts:47`) → `PUT
/api/v1/{plural}/{id}/_attachments/{property}` → `handleV1PutAttachment`
(`internal/dataentry/handlers_attachment.go:147`). This ticket adds no server
surface.
- **Sibling prior art:** TKT-RXFD5B built this exact widget for edit mode;
TKT-WLLRO7 added `max`. The staged mode is a third branch on the same component,
not a new one.
- **Two-phase create is already an established pattern in this very function.**
`handleSubmit` already does create-then-follow-up-write for relations:
`link_as=from` creates the reverse relation *after* the entity exists
(`DynamicForm.vue:1023-1035`), catching and warning on failure. Staged
attachment upload is the same shape, so this is not a new idea in the form.
- **Rejected alternatives** (detail on ticket): multipart create — atomic but
needs a body-shape branch, multipart size accounting, and a compensating delete;
staging namespace — needs its own ACL subject, GC, and quota.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*1. `FileWidget` staged mode.* Today `canEdit = mode === 'edit' && !disabled &&
!!entityType && !!entityId`. Add a parallel `canStage = mode === 'edit' &&
!disabled && !entityId` branch. In staged mode the widget keeps `stagedFiles:
File[]`, renders them from the same list markup (size via the existing
`formatSize`, image preview via `URL.createObjectURL`), and emits
`update:staged-files` instead of uploading. Object URLs are revoked on remove
and on unmount — a leak here is a real one. Capacity logic (`atCapacity`,
`isSingle`) is reused by counting `files.length + stagedFiles.length`.

*2. `DynamicForm` staging state + post-create upload.* Add `stagedFiles =
ref<Record<string, File[]>>({})`, fed by the widget's emit through
`FormFieldList` (which already forwards `attachment-changed`).

**Critical: staged files must NOT enter `formData`.** `payload.properties` is
built from `visibleWritablePropertiesForCommit()` (`DynamicForm.vue:376`), and
any key present there is sent to the server. A `File` object — or a fake path
string — in `formData` would be sent as the file property's value at create,
which is wrong (the server stamps that property itself, in
`stampPropertyNames`). Keeping staging in a *separate* ref is the design point,
not an implementation detail. It also keeps the dry-run
(`refreshStagedAffordances`, line 607, which posts `{...formData.value}`) clean.

Then in `handleSubmit`, upload each staged file sequentially against
`entity.id`. Sequential rather than parallel: uploads serialize on the server's
`writeMu` anyway, and `WriteAttachment`'s cap check is read-then-write
(non-atomic, relying on serialized writers — see
`internal/attachment/attachment.go` doc), so concurrent uploads to one property
are exactly the case its cap logic is not designed for.

**Exact statement order (RR-6ZAOMK).** The span between `create` (line 1014) and
navigation (line 1050) is not inert — it holds `dirty.value = false` (1037), the
embedded early-return (1043-1046) and `uiStore.success(...)` (1048), and the
uploads' position relative to each changes behaviour. Pinned order:

1. `entitiesStore.create(...)` → `surfaceWarnings(entity.warnings)`
2. **upload staged files**, continue-on-error, collecting `{filename, error}`
3. `dirty.value = false` **only if every upload succeeded** — otherwise stay
dirty so the navigation guard still protects the un-uploaded files
4. embedded early-return (`emit('inline-created', entity)`)
5. outcome-dependent toast — success, or a partial-success message naming the
failed files (see RR-Z7C3CY)

Uploads must precede the success toast: firing green "Entity created
successfully" and then a red upload error describes one user action with two
contradictory messages.

*3. Failure handling.* Wrap the upload loop. On failure: still navigate to the
created entity (it exists — hiding it would be worse), and surface the error.
Deliberately **unlike** the `link_as=from` precedent above, which only
`console.warn`s (`DynamicForm.vue:1032`) — a silently-dropped attachment is
user-visible data loss, so this gets a real error toast carrying the server's
problem+json detail, which `AttachmentError` already extracts
(`api/attachments.ts:31-39`).

*3b. Dirty tracking must include staged files (RR-EUX4BX).* Because staged files
deliberately stay out of `formData`, `checkDirty()`
(`DynamicForm.vue:1380-1388`) cannot see them — it serializes only `{formData,
relations, content}` plus `pendingCardChanges`. A user who attaches a file and
navigates away would hit neither guard (`if (!dirty.value) return true`, line
1596; `handleBeforeUnload`, ~1397) and lose it silently. `pendingCardChanges` is
the existing precedent for a non-`formData` dirtiness source; do the same:

```js
const hasStagedFiles = Object.values(stagedFiles.value).some((a) => a.length > 0)
dirty.value = currentData !== originalData.value || hasCardChanges || hasStagedFiles
```

*4. Embedded/inline-create path.* `props.embedded` returns early at line 1044
(`emit('inline-created', entity)`) **before** navigation. The upload must run
before that early return, or attachments staged in an inline-create modal are
silently dropped. This is the easiest thing in the whole ticket to get wrong.

**Files to modify:**
- `frontend/src/widgets/FileWidget.vue` — staged mode, object-URL lifecycle
- `frontend/src/widgets/types.ts` — `WidgetProps`/emit for staged files
- `frontend/src/components/forms/FieldRenderer.vue` — forward the new binding
- `frontend/src/components/forms/FormFieldList.vue` — forward the new binding
- `frontend/src/components/forms/DynamicForm.vue` — `stagedFiles` ref, upload
loop after create (both the embedded and navigating paths), error surfacing
- Tests: `frontend/src/widgets/widgets.test.ts`,
`frontend/src/components/forms/DynamicForm.test.ts`, new
`e2e/tests/attachments-create.spec.ts` + a `file` property and page-object
methods in `e2e/tests/fixtures.ts` / `e2e/pages/`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **User-selected files** (picker / drag-drop). Validated **server-side, exactly
as today** — sniffed MIME allowlist, sandboxed scan/transform, size cap, per
`docs/attachment-security.md`. The staged client holds bytes in memory and ships
them to the unchanged endpoint. **No client-side check is treated as a security
control**; any client-side size hint is a UX courtesy, and the 413 remains
authoritative.
- **The entity id used for upload.** Create mode passes `entityId === undefined`
(`DynamicForm.vue:1742`), and uploads use ONLY the server-returned `entity.id`,
so no client-side value can reach an attachment URL. Pinned by an explicit test.
**Correction (RR-QTCKCW):** an earlier draft claimed `entityId` holds the
`++new++` STAGED_ID sentinel that must be prevented from leaking. That is false
— `STAGED_ID`/`isStaged` (`stagedEntity.ts:12-15`) have no production consumer.
Do NOT "make the guard explicit" by assigning the sentinel into `entityId`; that
would manufacture the very leak the old wording imagined.

**Security-Sensitive Operations:**

- **Authorization is unchanged and still server-side.** Every upload hits
`attachmentWritePreflight` (`handlers_attachment.go:363`): uniform-404 read gate
→ entity/type check → file-property + hidden-property check → lock check →
`AuthorizeWrite("update")`, with an `OpDeniedWrite` audit row on deny, all
*before* the multipart body is parsed. Staging changes none of it. A client that
stages a file it may not upload gets a 403 at commit — the correct boundary.
- **No new attack surface by construction:** no new route, no unowned-blob
namespace, no ACL subject, no ID reservation primitive (the absence of which is
what forces this design at all).
- **Error text** shown inline is the server's own problem+json `detail`, already
written for client consumption by `writeV1Error`. No new channel, nothing
additional exposed.
- **Memory, not a vuln but worth stating:** staged files sit in browser memory
until save. Bounded by the property's `max` and the user's own picking.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** (numbers map to Acceptance Criteria above)

| AC | Level | Test |
|----|-------|------|
| 1 | e2e | `attachments-create.spec.ts`: create entity with a file, assert attached + property stamped |
| 1 | unit | `DynamicForm.test.ts`: create resolves → `uploadAttachment` called with `(type, returnedId, prop, file)` |
| 2 | unit | stage then remove → widget emits empty list; no upload on save |
| 3 | unit | `widgets.test.ts`: staged files count toward capacity; add control hidden at `max` |
| 4 | unit | `uploadAttachment` rejects with `AttachmentError(413)` → error surfaced, navigation still happens |
| 5 | unit | rejects with `AttachmentError(403)` → error surfaced, no success claim |
| 6 | unit | assert `uploadAttachment` is called with the server-returned `entity.id` |
| 7 | — | no change; existing behaviour untouched |
| RR-EUX4BX | unit | stage a file → `isDirty()` true; navigation guard prompts |
| RR-6ZAOMK | unit | failed upload → form stays dirty; no bare success toast |
| RR-Z7C3CY | unit | mixed batch (some succeed, some fail) → all attempted, one message names the failures |

Unit tests follow the existing `mount(FileWidget, { props: {...} })` convention
(`widgets.test.ts:508+`) and the `DynamicForm.test.ts` mocking style.

**Integration:** e2e is the real integration proof (real `rela-server`, real
store, real upload endpoint). Note `e2e/tests/fixtures.ts` currently declares
**no `file` property anywhere** — a `file` property must be added to the inline
fixture schema, plus page-object methods for the file control (the page-object
pattern is an eslint-enforced compile error to bypass — `e2e/tests/AGENTS.md`).
That fixture work is a real, easily-underestimated slice of this ticket.

**Edge Cases:**

- Create **fails** (validation 422) with files staged → no upload attempted;
staged files survive so the user can fix and resubmit without re-picking.
- **Embedded/inline-create** (`props.embedded`) → upload must run *before* the
early `emit('inline-created')` return at line 1044.
- Staging **more than `max`** → excess refused at pick time, consistent with
edit mode.
- Same file staged twice / same filename → server auto-suffixes on collision
(`resolveAttachName`); client need not dedupe.
- **Partial batch failure at `max > 1` (RR-Z7C3CY)** → continue-on-error: every
staged file is attempted, failures collected as `{filename, error}`, and ONE
message names them (`Entity created. 2 of 3 files failed: b.pdf (too large),
c.exe (type not allowed).`). Aborting the loop would leave later files
unattempted with nothing telling the user so; a bare per-request detail names no
filename, leaving "which of my files made it?" unanswerable.
- **Navigating away mid-upload** → `isCancelledFetch` handling already exists in
this catch block (BUG-6C3V) and must not be broken.
- **Empty staging** (no file picked) → the create path is byte-for-byte
unchanged; the common case must not regress.
- Object URL **revoked** on remove and unmount (no leak).
- File property that is **hidden by policy** in create mode → no staged control,
same as any hidden field.

**Negative Tests:** Oversize (413), rejected MIME (422), ACL deny (403), network
failure — each must show the server's detail, keep the created entity reachable,
and never report success. Silent failure is the specific thing this ticket must
not ship.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Staged file leaks into `formData` and is POSTed as the property value.**
The main correctness risk. → Separate `stagedFiles` ref, never merged into
`formData`; test asserts the create payload has no file-property key.
2. **Embedded path drops attachments** (early return before upload). → Called
out in the approach; explicit unit test.
3. **Partial failure feels broken to the user** — the accepted cost of Option A.
→ Loud inline error + land on the created entity. If it proves insufficient in
practice, the multipart-create option is documented and ready.
4. **e2e fixture has no `file` property today** → schema + page objects must be
added; scoped above so it isn't discovered mid-implementation.
5. **Object-URL leak** on preview. → Revoke on remove and unmount.
6. **Regressing the no-attachment create path** (by far the most common). →
Existing `DynamicForm` create tests must stay green untouched.

**Effort:** m — confirms the ticket estimate. Frontend-only, but spans 5 source
files plus non-trivial e2e fixture work.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/data-entry.md` — UI change: attachments can now be added while
creating an entity, not only when editing. Worth a line since the old
save-then-reopen workaround is what users learned.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`CLAUDE.md`~~ (N/A: no new cross-cutting pattern)
- [x] ~~`README.md`~~ (N/A: no project-level change)
- [x] ~~`docs/attachment-security.md`~~ (N/A: the security model is deliberately
unchanged — that is the whole argument for Option A)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Finding | Status |
|----|----------|---------|--------|
| RR-QTCKCW | significant | `++new++` sentinel premise factually wrong; AC6 tested a non-existent risk | addressed |
| RR-EUX4BX | significant | Staged files invisible to dirty tracking → silent loss on navigate-away | addressed |
| RR-6ZAOMK | significant | Upload placement vs. dirty-reset / success-toast unspecified | addressed |
| RR-Z7C3CY | minor | Multi-file partial-batch failure unspecified | addressed |

No critical findings. The three significant ones were all *omissions in the
plan* rather than flaws in the chosen approach — Option A itself survived
review. Notably RR-EUX4BX is a direct consequence of the plan's own central
decision (keep staged files out of `formData`), which is the kind of
second-order effect this review exists to catch.
