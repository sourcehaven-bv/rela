---
id: PLAN-1XW722
type: planning-checklist
title: 'Planning: App CSP: drop unsafe-inline and split the scaffold into external files'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: `appCSP()` drops `'unsafe-inline'` from script-src/style-src; the `rela apps
new` scaffold splits into index.html + app.css + app.js; the reference app
`tickets/apps/ticket-counter/` splits the same way (including its two `style=""`
attributes); app-authoring docs state the rule; tests pin both halves.

OUT: nonce or hash-based CSP (needs per-request HTML rewriting — see
"Alternatives"); `unsafe-hashes` for `style=""` attributes; the SPA's own CSP;
every other CSP directive (`connect-src 'none'`, path-scoping, `form-action`,
`frame-ancestors`) is unchanged.

**Acceptance Criteria:**

1. AC1 A freshly scaffolded app loads and runs under the strict CSP with no
violations → `rela apps new`, serve, load in a real browser.
2. AC2 The reference app renders identically → compare computed styles for the
two former `style=""` attributes.
3. AC3 `appCSP()` contains no `'unsafe-inline'` → assertion in
`TestAppCSP_PathScopedNoEgress`.
4. AC4 An inline `<script>` / `onerror=` in an app is blocked → inject both in
a browser under the served CSP.
5. AC5 External `.css`/`.js` serve with correct content types → existing
`appContentTypes` map; verified over HTTP.
6. AC6 Docs tell app authors to use external files and classes.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the design question was settled empirically (see below)
rather than by survey.

**Existing Solutions:**

- `TKT-VEJ39W` specifies the current header and marks which parts are
load-bearing: path-scoping every resource directive, `connect-src 'none'`,
`form-action 'none'` + sandbox. `unsafe-inline` is NOT among them — it was there
to let single-file apps work, not as part of the boundary.
- `appContentTypes` (`internal/dataentry/apps.go:206`) already maps `.js`/`.css`
to fixed content types, with a comment noting a correct type is load-bearing
under nosniff. So external assets were always a supported shape; nothing new is
needed to serve them.
- The reference app already avoids `innerHTML` in favour of `textContent`, with
a comment saying why. This change makes the CSP back that discipline up.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Drop `'unsafe-inline'` from the two directives, and make the scaffold emit the
layout that works under the result:

```go
"script-src " + base,
"style-src " + base,
```

`ScaffoldAppWithFS` writes three files instead of one. index.html is written
LAST, because `appExists()` keys on it — writing it after its assets means a
half-written scaffold is not yet a live app.

**Alternatives rejected:**

- *Nonce-based CSP.* Real protection, but apps are static operator files
streamed verbatim by `openAppEntry`; a nonce must be a per-response attribute on
every inline tag, so this turns static file serving into an HTML
parse-and-rewrite pipeline — a new security-relevant component, for a Low
finding. It also only covers `<script>`/`<style>` elements; `style=""`
attributes would still need `unsafe-hashes`.
- *Per-app opt-in (`<meta name="rela-app:strict-csp">`).* Backwards compatible,
but two CSP paths to reason about and test, and the apps that most need the
protection are the least likely to set the flag.
- *Leave it and document.* This was my recommendation while the scaffold was
the blocker. Once the scaffold is fixed, the argument for keeping
`unsafe-inline` is gone: the cost was compatibility, and there is no in-repo app
or generated app that needs it.

**Files to modify:**

- `internal/dataentry/apps_handler.go` — the CSP + its doc comment
- `internal/dataentry/apps_test.go` — assert no `unsafe-inline`
- `internal/projectsetup/app.go` — three-file scaffold
- `internal/projectsetup/app_test.go` — updated + inline-free guard
- `internal/cli/apps.go` — report the new files, mention the rule
- `tickets/apps/ticket-counter/{index.html,app.css,app.js}` — split
- `docs-project/entities/guides/GUIDE-data-entry.md` — the authoring rule
(regenerate `docs/` with `just docs`)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

No new input. The CSP is emitted from `appBaseURL`, which already refuses a Host
header containing characters that could inject CSP tokens.

**Security-Sensitive Operations:**

The header IS the operation. Removing a source expression only narrows what the
app may load, so the failure direction is "a legitimate app resource is
blocked", not "something unsafe is permitted" — which is why the acceptance work
is mostly about proving apps still WORK.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

Unit: `TestAppCSP_PathScopedNoEgress` gains an `unsafe-inline`/`unsafe-eval`
assertion (AC3). `TestScaffoldApp_NoInlineCodeOrStyles` fails if the scaffold
ever reintroduces an inline block, a `style=""`, or an `on*=` handler (AC1's
regression half).

Browser (headless Chrome, real served CSP) — a unit test cannot show that a
browser accepts the policy, which is the actual claim:
- scaffolded app: external css/js applied, `window.rela` present (AC1)
- reference app: computed padding/margin match the old inline values (AC2)
- injected `<script>` and `onerror=` both blocked (AC4)
- control on the OLD CSP: the same injection RUNS — so the test discriminates

**Edge Cases:**

- `style=""` attributes — blocked (need `unsafe-hashes`), so the reference app's
two were moved to classes. Documented for authors.
- `on*=` inline handlers — blocked by `script-src-attr`; verified.
- Asset names must not start with `_`: the handler reserves that prefix for
binary-served endpoints, so `app.css`/`app.js` (no underscore) are correct.
- A half-written scaffold: index.html is written last so the folder is not a
live app until its assets exist.
- Existing third-party apps that inline WILL break. Accepted — see Risks.

**Negative Tests:**

Inline code fails VISIBLY: a CSP violation in the browser console naming the
directive. It does not fail the request or break the host, so the docs point
authors at the console.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *An existing app that inlines stops working.* The real cost of this change.
Mitigated: the only in-repo app is the reference one (split here), the scaffold
is fixed, the failure is a clear console violation rather than silence, and the
docs say what to do. Explicitly accepted rather than softened with a per-app
opt-out, which would leave the protection off exactly where it matters.
- *The strict CSP breaks something not covered by the two apps I tested.*
Mitigated by testing in a real browser rather than asserting on the header
string, and by leaving every other directive untouched.

**Effort:** s

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] docs/metamodel.md - New metamodel features
- [x] docs/cli-reference.md - New/changed commands
- [x] docs/data-entry.md - UI changes
- [x] CLAUDE.md - New patterns or conventions
- [x] README.md - Project-level changes
- [x] N/A - Internal change, no user-facing docs needed
-->

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A — the design risk here was "does a real browser
accept this", which was settled by building it and loading it before writing the
ticket, rather than by review.
