---
id: TKT-3DBK6I
type: ticket
title: 'Operator customisation hooks: serve + inject custom.css/custom.js, @layer cascade fix, isCustomElement'
kind: enhancement
priority: medium
effort: l
status: done
---

## Scope

Items 1–3 and 6 of the operator-customisation-hooks proposal. Items 4–5 (`rela-`
class hooks on the next-action component, and the `<rela-slot>` consumer) are
**blocked** on the next-action feature and are NOT in this ticket.

1. Serve `custom.css` / `custom.js` from the project root at `/_custom/*`,
traversal-resistant via `os.OpenRoot`, never in the entity store (so it works
across storage backends). Absent file → 404, no injection, no error.
2. Inject `<link>` / `<script type="module">` into the SPA's `index.html`,
intercepting the `spaHandler` response (`internal/dataentry/router.go`). **With
the `@layer` fix below** — the naive version does not work.
3. Add `isCustomElement: (tag) => tag.startsWith('rela-')` to the Vue compiler
config so an unregistered `<rela-slot>` does not warn on every render.
4. Document the three-tier contract, the hooks, and the gotchas.

## VERIFIED FINDINGS (measured, not assumed)

The proposal flagged the CSS cascade as the one item to test before committing.
It was tested against a real production build plus a browser, and **the original
premise is false**.

**Assumed:** no `<link>` in `index.html`; CSS is runtime-injected by the
bundler, so an operator `<link>` in `<head>` lands last and wins.

**Actually:** `npm run build` emits a real `<link rel="stylesheet">` into
`<head>`, *and* emits **19 CSS files**. Only `index-*.css` is linked eagerly;
the other 18 are route-level chunks that Vite's `__vitePreload` helper appends
at runtime via `document.head.appendChild(link)` (confirmed in built output).

Measured DOM order after navigating to a route:

```
[0] /assets/index-*.css
[1] /_custom/custom.css     <- operator
[2] /assets/ListView-*.css  <- appended on route nav, WINS
```

Confirmed at the **rendering** level, not just DOM order: with an
equal-specificity tie (0,1,0 both sides) `getComputedStyle` resolved to the
route chunk. `operatorWins: false`.

Consequence: operator CSS beats `index.css` (global tokens) but **loses ties to
every route view** (ListView, KanbanView, EntityView, FormView, DocumentView,
…). The failure is route-dependent — a skin that works on the dashboard silently
stops working after navigating to a list view, which reads as a rela bug.

This also inverts the proposal's item-4 specificity argument ("careless operator
CSS loses, deliberate wins by adding specificity"): at equal specificity the
operator also loses, purely on source order.

### Cascade decision (agreed with maintainer)

**Option 2 + 4:** wrap rela's own CSS in `@layer` — unlayered operator CSS
outranks *all* layered CSS regardless of source order — and document the
limitation honestly. Rejected: injecting before `</body>` (still loses to
later-loaded chunks) and re-appending the link on route change (contradicts "no
route-change events available to outside JS").

No `@layer` currently exists anywhere in `frontend/src/` — this is greenfield.

**OPEN QUESTION (blocks implementation):** layer scope — wrap *all* rela CSS in
`@layer rela { … }` (robust, touches every stylesheet and all 19 build outputs)
vs. layering only the route-chunk CSS that actually beats operators today
(narrower, leaves edge cases and reintroduces order-dependence).

### isCustomElement — verified

The predicted Vue warning reproduces verbatim in a real SFC:

```
[Vue warn]: Failed to resolve component: rela-slot
If this is a native custom element, make sure to exclude it from component
resolution via compilerOptions.isCustomElement.
```

Proven causal by a revert-and-rerun control: warning present with bare `vue()`,
absent with `isCustomElement`. Same test, only the config differed.

Two findings the proposal missed:

- **`vitest.config.ts` has the same bare `plugins: [vue()]`.** Without fixing it
too, every unit test rendering a `rela-` element warns even after the
`vite.config.ts` fix lands. (Already applied on the working tree.)
- **`<rela-editor>` IS in the main bundle** — `relaEditor.ts` is imported by
`components/forms/MarkdownEditor.vue`, so the main compiler config *does* reach
it (the proposal assumed a separate build insulated it). Still safe:
`isCustomElement` only suppresses Vue's component *resolution*; the
`customElements.define()` call at `relaEditor.ts:308` is untouched. Its 15 tests
pass. Right conclusion, wrong reason.

### Test-methodology trap (record for the implementer)

A first probe using a **runtime string `template`** showed the warning
persisting even with the fix applied — a false negative. Runtime-compiled
templates never see build-time `compilerOptions`. Any regression test for this
**must be a `.vue` SFC**; a string-template test will fail confusingly forever.

## Baseline

Full frontend suite green: 96 files / 1531 tests. Typecheck clean, lint 0 errors
(92 pre-existing warnings).

Pre-existing flake, NOT caused by this work: `EnvironmentTeardownError` in
`DynamicForm.test.ts` (vitest worker RPC teardown race). Reproduced once in 5
runs on unmodified `develop`; 1-then-0-0 with the change.

## Acceptance criteria

- AC1 `/_custom/custom.css` and `/_custom/custom.js` serve from the project root
when present; 404 when absent; path traversal is rejected.
- AC2 Tags are injected into `index.html` only when the corresponding file
exists; a stock deployment's HTML is byte-unchanged.
- AC3 Operator CSS wins an equal-specificity tie against a **route-chunk**
selector after client-side navigation (the case that is broken today).
- AC4 An unregistered `<rela-slot>` renders inert and emits no Vue warning, in
both the app build and the unit-test config.
- AC5 `<rela-editor>` still registers and functions.
- AC6 Docs state the three-tier contract, the verbatim stability disclaimer, and
both known gotchas.

## Security

Same-origin static files from the operator's own project directory; no new
privilege. If a CSP is ever applied to the SPA route it must permit these paths
— serving files rather than inlining means a path allowance, not
`unsafe-inline`. Operator errors surface through the existing
`app.config.errorHandler` / `unhandledrejection` logging in `main.ts` and will
look like rela bugs in reports — hence the "remove custom.js and retry before
filing a bug" line.
