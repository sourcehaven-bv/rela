---
id: PLAN-0E3J9A
type: planning-checklist
title: Planning
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: `frontend/src/app-editor/` (the `<rela-editor>` Custom Element, its toolbar,
its `@` completion, its stylesheet), the standalone build
`frontend/vite.editor.config.ts`, and the Go side that serves the result
(`internal/dataentry/apps{,_editor,_handler}.go`).

OUT: the element's public contract. `value`, `placeholder`, `readonly`, `input`,
`change`, `focus()` do not change. The swap seam exists so this move is
invisible to an app, and no app may need an edit.

OUT: title resolution for entity references. See Security below.

**Acceptance Criteria:**

1. An app needs no change: the six-item contract behaves identically.
2. The two editors cannot serialize a body differently.
3. The editor works under the real app CSP in a real browser.
4. No webfont ships.
5. `.value` is churn-free: a body the app only displayed comes back byte for
byte.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-AV732E (written for TKT-3I9DDY; its editor survey and
serialization findings apply unchanged here, which is why no second survey was
run).

**Existing Solutions:**

- Milkdown 7.22.1 is already a dependency (`@milkdown/kit` bundles commonmark,
gfm, history, listener, slash, block, cursor, trailing, tooltip). No new
dependency.
- The SPA editor (`frontend/src/components/forms/milkdown/`) is the reference
implementation, and more than that: its modules are meant to be imported here
rather than imitated. RR-9PTXV0 recorded the fork as a defect and deferred the
extraction to this ticket.
- `internal/dataentry/apps_sdk.go` already exposes `search` on the bridge, which
is the only host call the `@` menu needs.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Import every load-bearing module from the SPA editor; write locally only what
needs a framework there. The bundle is a plain IIFE with no Vue, so the toolbar
and the `@` menu are built through the DOM API, and anything reaching for Vue or
the axios API layer cannot be imported into it.

Shared (imported, not copied): `editorCommands`, `activeFormats`,
`commandAvailability`, `tableCommands`, `entityRefNode`, `serializerContract`,
`writeBackGuard`, `mentionQuery`.

Extracted so the IIFE can reach them: `editorIcons.ts` (glyph geometry, rendered
by both `BlockIcon.vue` and the plain-DOM toolbar) and `rankMentions.ts`
(`rankByIdMatch` currently sits in `useMentionMenu.ts`, which imports Vue and
axios).

Two more extractions were added after design review, both because the first pass
had left a copy behind: `mentionMenuState.ts` (RR-YLO6CG — the menu's state
machine, which had been duplicated with only the transport and rendering
differing) and `editorPreset.ts` (RR-3KSGIS — the plugin set and serializer
options, which each editor had been computing separately with a comment in both
files asserting they matched).

Local: `relaToolbar.ts`, `relaMentionMenu.ts`, and the element itself.

Alternatives rejected:

- *Shadow DOM.* ProseMirror works in a shadow root, but the app's own
`_rela.css` tokens would not reach it and the editor is meant to look like the
app it sits in. Light DOM, as today.
- *A second entity-picker modal.* The SPA has one; building another here means a
second search UI to keep in step. The toolbar button types the `@` trigger
instead, so there is one menu reached two ways.
- *Keeping the write-back guard off `.value`.* Rejected: the getter IS the app's
save path, so the guard has to sit on it.

**Files to modify:**

- `frontend/src/app-editor/relaEditor.ts` (rewrite), `relaEditorTheme.css`
(rewrite), `relaBacktick.ts` + test (delete), `relaEditorFont.css` (delete)
- `frontend/src/app-editor/relaToolbar.ts`, `relaMentionMenu.ts` (new)
- `frontend/src/components/forms/milkdown/editorIcons.ts`, `rankMentions.ts`
(new); `BlockIcon.vue`, `useMentionMenu.ts` (render/import from them)
- `frontend/src/styles/markdown-content.css` (drop the EasyMDE preview block),
`markdownContentMirror.test.ts` (delete: the copy it guarded is gone)
- `frontend/vite.editor.config.ts`, `frontend/package.json`
- `internal/dataentry/apps.go`, `apps_editor.go`, `apps_handler.go`, `apps_test.go`
- `e2e/tests/fixtures.ts`, `e2e/pages/app-host.page.ts`, `e2e/tests/apps.spec.ts`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *Markdown set by the app via `.value`* — parsed by Milkdown into a ProseMirror
document. Never interpolated into HTML by this code. An entity reference is
recognised by SHAPE only (`isValidEntityRefId`, an allowlist
`^[A-Za-z0-9][A-Za-z0-9_-]*$` mirroring `internal/entity.ValidateID`), never by
consulting the graph — a reference to something unreadable or nonexistent must
still round-trip untouched, or the editor silently rewrites it on save.
- *Bridge search results* — each row's `id` is re-validated against the same
allowlist before it is offered, so a row the editor could not write as a
reference is not shown at all.
- *The `placeholder` attribute* — set as `textContent`, never HTML.

**Security-Sensitive Operations:**

- **Entity-reference titles are NOT resolved here (BUG-R9EHKV).** The SPA gets
titles from the server's per-principal `mentions` map, computed through
`visibility.Reader.Filter`. The app bridge has no such endpoint. Deriving a
title from `rela.get`/`rela.search` client-side would route around the read gate
and turn the editor into a second read path, so references render as bare IDs.
The bridge's own results are already ACL-scoped, so listing IDs from them
discloses nothing new — but a row must not claim more than the ID.
- **The app CSP must not be loosened.** `script-src`/`style-src` carry no
`'unsafe-inline'` deliberately, as defence-in-depth against an app's own bugs.
The editor adapts: its stylesheet is a served FILE reached by `<link>`, never an
injected `<style>` (which would be blocked outright, leaving the editor unstyled
with only a console violation to show for it).
- **No new reserved path, and one removed.** Dropping Font Awesome removes
`_rela-editor.woff2` and with it an `Access-Control-Allow-Origin: *` exception
that existed solely because a sandboxed iframe is null-origin.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. (contract) `relaEditor.test.ts` drives the REAL editor — no mock, unlike the
EasyMDE version which had to fake CodeMirror because it will not mount under
happy-dom. Edits are made through the toolbar, which is a real user action end
to end and needs no editor handle the contract does not expose.
2. (no drift) REVISED after design review (RR-3KSGIS). The original plan said
"the modules are imported, so there is nothing to compare", which is weaker than
it sounds: each editor still builds its own Milkdown instance, and the plugin set
is what decides the output. `editorPreset.ts` now holds the plugin set and the
serializer options, and `editorPreset.test.ts` pins its output over block
constructs AND greps both editor sources to fail if either rebuilds the markdown
stack locally. `mentionMenuState.test.ts` does the same job for the menu.
`editorIcons.test.ts` asserts every command in the shared catalogue has a glyph,
which would otherwise rot silently into a blank button.
3. (CSP) `apps.spec.ts` mounts the editor in the sandboxed iframe under the real
path-scoped header and asserts the served stylesheet applied, a command runs end
to end, and no violation is raised.
4. (no webfont) e2e 404 on `_rela-editor.woff2`; a Go test on the same path; a
unit assertion that every toolbar button drew an inline SVG.
5. (churn) `relaEditor.test.ts` round-trips a table and a list through `.value`.

**Edge Cases:**

- Element disconnected while the async `Editor.create()` is still in flight —
must tear down rather than leave an editor mounted in a detached shell.
- `.value` set before connection (no editor yet) and after disconnection.
- `readonly` toggled after mount; a toolbar command pressed while readonly.
- A slow bridge response landing after the user has moved on, or after the menu
closed — the bridge offers no cancellation, so staleness is handled by
generation rather than abort.
- A search row whose `id` is not a writable reference.
- Two `<rela-editor>` elements on one page — the stylesheet must link once.
- No bridge at all (editor used outside an app): completion is simply absent,
and the entity-reference button is not drawn.

**Negative Tests:**

- The editor failing to construct must leave a working plain `<textarea>`, not a
dead element.
- A command that cannot apply where the cursor is must be refused by the handler,
not merely greyed: `aria-disabled` does not prevent activation.
- `.value` set programmatically must emit NO `input` (native `<textarea>`
semantics; the opposite caused an autosave loop that froze the Today app).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *The recorded CSP blocker.* PLAN-JQ2FBA held this ticket back on the claim
that `style-src` without `'unsafe-inline'` blocks editor chrome. Mitigation:
test the claim before building anything. Done, twice — a probe server
replicating `appCSP` exactly, then a real Milkdown bundle under the same header.
The claim is false: `style-src` governs stylesheets and the `style` ATTRIBUTE,
while ProseMirror writes DOM style PROPERTIES (CSSOM), which is permitted. An
e2e test now pins it.
- *Bundle size.* Milkdown is larger than EasyMDE. Mitigation: dropping Font
Awesome returns most of it — CSS 52KB→27KB and a 77KB font removed against JS
337KB→494KB. The editor is opt-in per app, so only apps that embed it pay.
- *Event semantics drifting from a native control.* `input` must fire per
keystroke and carry the post-edit value; `change` only on blur after a real
edit. Mitigation: explicit tests for both, including the toolbar-refocus case.
- *Effort: l.*

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] CLAUDE.md — `internal/dataentry/CLAUDE.md` records the editor's serving
contract, and currently states the CSP claim this ticket disproves plus a
webfont path it removes. `frontend/CLAUDE.md` gains the app-editor section and
the share-don't-mirror rule.
- [x] docs/data-entry.md — the custom-apps chapter gains a section documenting
`<rela-editor>` for app authors. This was NOT in the original plan, which
assumed the element was out of the guide's scope; it turned out the guide
already documents custom apps thoroughly and the element was simply missing
from it, a gap dating to TKT-5F9V56.
- [x] N/A for docs/metamodel.md, docs/cli-reference.md, README.md — no
metamodel, CLI or project-level surface changes.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Eight findings, no critical. All addressed.

| ID | Severity | Finding |
|----|----------|---------|
| RR-YLO6CG | significant | Mention-menu state machine was forked, not shared |
| RR-FMNVRS | significant | "No webfont ships" was untestable by construction |
| RR-ZW2IEO | significant | Churn-freedom tested on the path where it holds by construction |
| RR-V8P47K | minor | Stylesheet link had no error path |
| RR-JE7J58 | minor | `scopeScalesToEditor` transform undocumented and unasserted |
| RR-3KSGIS | minor | AC2 argued from the import graph rather than tested |
| RR-HB2VCC | nit | `placeholder=""` overridden by the default |
| RR-LF36QW | nit | Reference button dirtied the document before the user committed |

The three significant ones share a shape worth recording: each was a claim this
plan made that no test could have falsified. The fork claim was checked against
an import list rather than against the two files; the webfont claim was checked
against a served path the build config had made unreachable; the churn claim was
checked on the branch where the guard short-circuits. The fixes are
correspondingly about moving each assertion to where the property can actually
fail — a shared module, the build output, and the edited path.

Two further extractions followed, neither in the original plan:
`mentionMenuState.ts` (the menu's state machine) and `editorPreset.ts` (the
plugin set and serializer options, which decide the bytes and which both editors
had been computing separately).
