---
id: IMPL-HI0CB1
type: implementation-checklist
title: Implementation
status: done
---

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The element tests drive the REAL editor rather than a mock. The EasyMDE version
had to fake CodeMirror because it will not mount under happy-dom; ProseMirror
does, so every assertion now runs against the editor an app actually gets. Edits
are made through the toolbar, which is a real user action end to end and needs
no handle the public contract does not expose.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

1. *An app needs no change.* `relaEditor.test.ts`, 29 tests against the real
editor: value round-trip before and after connect, silent programmatic sets,
per-keystroke `input` carrying the post-edit value, `change` on blur only after
a real edit, readonly at mount and on change, teardown and reconnect,
placeholder visibility.

2. *The two editors cannot serialize a body differently.* `editorPreset.test.ts`
pins the preset's output over block constructs, pins that exactly
`remarkPreserveEmptyLinePlugin` and nothing else is dropped, and greps both
editor sources to fail if either rebuilds the markdown stack locally.
`mentionMenuState.test.ts` does the same job for the menu's staleness rules.

3. *The editor works under the real app CSP in a real browser.* `apps.spec.ts`,
10 tests in the sandboxed iframe under the real path-scoped header: the served
stylesheet applied (asserted via computed style, including a rule that can only
come from the concatenated `markdown-content.css`), a command runs end to end,
and zero CSP violations across mount, layout and editing. The demo app's own
deliberate probe violations are filtered by URL so the assertion means what it
says.

4. *No webfont ships.* The build itself now fails on `@font-face`, an inlined
font `data:` URI, or a font `url()` in either artifact — verified by appending
an `@font-face` and watching it fail. Plus a Go test and an e2e test that the
old reserved path 404s with no CORS header, and a unit assertion that every
toolbar button drew an inline SVG.

5. *`.value` is churn-free for an unedited body.* Two tests: one that an unedited
body comes back byte-identical (using only constructs the round-trip would
rewrite, so it fails if the guard is bypassed), one that an edit reserializes
the whole body (asserting the behaviour rather than leaving it to be found).

Also run: the full frontend suite (2660 tests), the full e2e suite (298 passed,
8 skipped), `go test ./internal/dataentry/`, and the editor e2e block three
times over to confirm a race I had introduced in the page object was gone.

Edge cases verified by test: disconnect during async create, `.value` before
connect and after disconnect, readonly toggled after mount, a command pressed
while readonly, a slow bridge response landing after the user moved on or after
close, a search row whose ID is not a writable reference, two elements on one
page linking the stylesheet once, no bridge at all, and the editor failing to
construct falling back to a plain textarea.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**On DRY specifically**, since this ticket is mostly about it. Three modules
were extracted, each because a copy already existed or was about to:
`editorIcons.ts` (glyph geometry, rendered by both a Vue component and a
plain-DOM builder), `mentionMenuState.ts` (the menu's state machine) and
`editorPreset.ts` (the plugin set and serializer options, which decide the bytes
an editor writes). `rankMentions.ts` was split out of `useMentionMenu.ts` so the
IIFE could reach it without pulling in Vue and axios.

Nothing was extracted for its own sake: the toolbar and the menu RENDERERS stay
separate, because one is a Vue component and the other is DOM API calls, and
merging them would mean a framework shim in a bundle that deliberately has no
framework.

**Security** was reviewed separately against rela's own invariants (read gate,
app CSP, input validation, XSS sinks, the removed CORS exception) and returned
no findings. The load-bearing decisions: entity references render as bare IDs
because the app bridge has no per-principal mentions endpoint and deriving a
title would route around the read gate (BUG-R9EHKV); the stylesheet is a served
file rather than an injected `<style>` because the app CSP carries no
`'unsafe-inline'`; and removing the webfont removed an
`Access-Control-Allow-Origin: *` exception along with it.
