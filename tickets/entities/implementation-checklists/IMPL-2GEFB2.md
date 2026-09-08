---
id: IMPL-2GEFB2
type: implementation-checklist
title: 'Implementation: App CSP: drop unsafe-inline and split the scaffold into external files'
status: done
---

<!-- @managed: claude-workflow v1 -->

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

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Unit tests alone cannot support the claim here — "the browser accepts this
policy and the app still works" is not observable from the header string. So
every acceptance criterion was checked by building the binaries, serving a real
project, and driving headless Chrome.

**AC1 — a scaffolded app works out of the box.** `rela apps new mydash` wrote
index.html + app.css + app.js. Served under the strict CSP (`script-src <base>;
style-src <base>`, no `unsafe-inline`):

| Observation | Value |
| --- | --- |
| `app.css` applied | h1 `17.5px` (1.25rem), body padding `21px` (1.5rem) |
| `.muted` colour resolved | `rgb(102, 102, 102)` |
| `_rela.js` + `app.js` loaded | `typeof window.rela === "object"` |
| scripts / stylesheets | `["_rela.js","app.js"]` / `["_rela.css","app.css"]` |
| violations during load | none |

**AC2 — reference app renders identically.** The two former `style=""`
attributes are now classes; computed values match what the inline attributes set
(root font-size is 14px):

| Element | Was | Now |
| --- | --- | --- |
| `#table` padding | `0.5rem 1rem` | `7px 14px` via `.counts-table` |
| `#refresh` margin-top | `1rem` | `14px` via `.refresh-button` |

`app.js` also *ran*: `#status` moved from "Connecting…" to "Loading tickets…",
which only happens inside `load()`.

**AC4 — the protection is real.** Injected both XSS shapes into a served app:

```js
d.innerHTML = '<img src=x onerror="window.__xss=1">';   // the innerHTML vector
s.textContent = 'window.__xss2 = 1;';                    // injected <script>
```

Result: `{inlineEventHandlerRan: false, injectedScriptRan: false, violations:
["script-src-elem <- inline", "script-src-attr <- inline"]}`.

**Control — the test discriminates.** The same injection against a binary built
from the CURRENT CSP: `{inlineScriptRan: true, violations: []}`. So the new
policy is what blocks it, not something else.

**AC5** — `app.js` → `200 text/javascript; charset=utf-8`, `app.css` → `200
text/css; charset=utf-8` over HTTP.

**Mutation testing** (both applied to real code, line-checked):

| Mutation | Result |
| --- | --- |
| Inline `<style>` put back in the scaffold template (app.go:116) | `TestScaffoldApp_NoInlineCodeOrStyles` reddens |
| `'unsafe-inline'` restored in `appCSP` (apps_handler.go:49-50) | `TestAppCSP_PathScopedNoEgress` reddens on both directives |

## Two real breakages the browser found (and static reading did not)

Both would have shipped as silent, visual-only failures. Neither was visible in
the diff, in any unit test, or in the CSP header string.

**1. The `<rela-editor>` bundle rendered completely unstyled.** `_rela-editor.js`
calls `ensureStylesInjected()`, which built a `<style>` element holding 44KB of
concatenated CSS (Font Awesome + EasyMDE + theme) and appended it to
`document.head`. Under the strict CSP that element is BLOCKED: it lands in the
DOM, but `style.sheet === null` and nothing applies. Observed
`.CodeMirror` inheriting the app's sans-serif instead of monospace, and the
toolbar glyphs gone.

Fixed by emitting the CSS as a build artifact (`emitEditorCSS` plugin in
`vite.editor.config.ts`) served at a new reserved endpoint `_rela-editor.css`,
with the bundle injecting a `<link>` instead. A `<link>` is a resource load that
the path-scoped `style-src <base>` already permits — no CSP widening. Same shape
as the existing `_rela-editor.woff2` endpoint.

After the fix, in-browser: `{violations: [], styleTag: "LINK",
styleSheetApplied: true, styleRuleCount: 919, cmFontFamily: "monospace"}`, and
12 Font Awesome elements resolving to family `FontAwesome` with real glyph
content.

Note this also shrank the JS bundle 382KB → 337KB, since the CSS is no longer a
string literal inside it.

**2. The e2e demo app would have stopped running.** `E2E_DEMO_APP_HTML` in
`e2e/tests/fixtures.ts` carried the CSP probe and the bridge calls in an inline
`<script>`. Blocked under the new policy, so every assertion in `apps.spec.ts`
would have failed against a blank page rather than the behaviour under test.
Split into a sibling `app.js` written alongside `index.html`.

## Test-filter mistake worth recording

I first ran the new endpoint's mutation with `-run "TestApp"` and read the
resulting `ok` as "the mutation did not break anything". The subtests live under
`TestHandleV1App`, which that pattern does not match — they never ran. Re-run
with the right filter, the mutation reddens exactly the two new subtests
(404 on the endpoint; missing ETag). A passing filter that selects nothing looks
identical to a passing test.

One test-authoring bug worth recording: the inline guard first matched the raw
strings `<style>`/`<script>` and failed on the template's own *comment*
explaining the rule. Fixed by stripping HTML comments before matching — the
comment is worth keeping, since it is what tells an author editing the template
why inlining is not an option.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
