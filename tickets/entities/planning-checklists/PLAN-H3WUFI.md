---
id: PLAN-H3WUFI
type: planning-checklist
title: 'Planning: rela-docs phase 3 (Tier B): screenshot{} island — chromedp capture of the seeded data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Phase 3 (Tier B) of FEAT-G4VO53. Adds the **`screenshot{}` statement-island**
to the phase-2 doc language: a headless-Chrome capture of the data-entry SPA
rendering a **seeded** entity, embedded in the manual as `![](fig.png)`.

**Proven by a spike** (this session): an in-process data-entry server (real
`appbuild.Discover` wiring over a temp project, in-memory bleve) + the built Vue
SPA + `chromedp` → a real styled PNG of the "Edit Ticket" form. The whole
pipeline works; the spike's only failures were fixable details (a dangling
`data-entry.yaml` view ref, an unstamped principal, a wrong entity dir name).

**Corrections the spike + design-review forced vs. the original design:**
- **bleve is fine.** The original "no-bleve, hand-wire 10 collaborators" constraint
  was self-imposed and pointless next to a Chrome dependency. Use
  `appbuild.Discover` over a temp project — one call, real production wiring.
- **The seed is a temp PROJECT DIR, seeded THROUGH the store (not hand-written
  markdown) — resolves DR-S2.** The SPA needs `metamodel.yaml` + `data-entry.yaml`
  (+ acl.yaml) on disk, so copy those schema/config files into a temp dir and
  `Discover`. But the entities are seeded by **replaying the manual's `create`/
  `link` calls against the temp project's `store.Store`** — NOT by a second
  markdown-emitting path (which would duplicate/drift from fsstore's on-disk
  format: dir plural↔singular, frontmatter key order, `FROM--type--TO.md`). One
  seed-application function drives both the phase-2 in-mem store AND the fsstore
  temp store, so they cannot diverge. (Seed raw via `store.CreateEntity`, NOT
  entitymanager — avoids automations mutating the fixture, e.g. status→checklist.)
- **Annotation anchor = the EXISTING `#field-<prop>` id — NO SPA change needed
  (corrects DR-C1).** The review found my "no `#field-<prop>` hook" claim was
  WRONG: `FieldRenderer.vue:41` computes `field-${property}` and stamps it as the
  widget's DOM id (`FieldRenderer.vue:88 :id="fieldId"`). So `#field-title` exists
  on the input control today. `FieldShell` has NO access to `field.property`, so
  the original "add data-field to FieldShell" was unbuildable. Decision: **anchor
  on `#field-<prop>` (the input control), zero SPA edit.** (If we later want the
  wider label+help+error box, add a `dataField` prop to FieldShell + pass it from
  FieldRenderer — deferred; the input-control box is fine for v1 arrows.)
- **`as="role"` = a REQUEST-INSPECTING resolver + per-capture role header
  (resolves DR-S1).** `PrincipalResolver` is invoked **per request** (router.go:110/287),
  so ONE reused server serves different roles across islands IF the harness sets a
  role header per navigation and the installed resolver maps header→role→principal
  (a principal assigned that role in acl.yaml). NOT a fixed single principal (the
  original plan's error). The harness sets the header via chromedp before each
  capture. The default `unknown` principal is rejected by a real Declarative ACL,
  so a role (default: the first non-everyone role, or an explicit `as=`) is always
  stamped.
- **Renderability gate (resolves DR-S4 — the fail-OPEN hole the spike hit).** The
  spike captured a "Failed to load entity" error state — a valid PNG of a broken
  form. Before capturing, the harness MUST assert the entity actually rendered:
  `WaitVisible` on a known `#field-<prop>` for a field on the form AND assert the
  SPA's error boundary is absent. The "entity failed to load" component gets a
  stable `data-testid` (small SPA edit) so the harness can fail-loud on it.

IN scope:
- **`screenshot{}` resolver** in the `doc.*` module: `screenshot{ view, type,
  entity, as, arrows, box, out }`. Emits `![alt](out)` markdown; writes the PNG
  next to the manual output.
- **A capture harness** (new package, see Approach): stand up the temp-project
  data-entry server, drive chromedp to the right SPA route, capture full-page or
  element-clip PNG.
- **Arrow-with-text annotations** anchored to `#field-<prop>` (schema fields,
  fail-loud if absent) or `@button:`/`@role:` (ARIA), drawn via an injected DOM
  overlay before capture. The overlay JS is built by **`json.Marshal`-ing the
  operator text into a JS string literal** and splicing that literal into the
  `chromedp.Evaluate` expression — `chromedp.Evaluate` has NO args channel
  (corrects DR-C2), so this is the safe technique (`json.Marshal` of a Go string
  escapes quotes, `</script>`, backslashes, U+2028/2029).
- **A `data-testid` on the SPA's "entity failed to load" boundary** so the
  renderability gate can detect a broken capture (the only SPA edit).
- **Fail-loud, no degradation:** no Chrome → `BuildError`; SPA not built →
  `BuildError`; unknown field anchor → `BuildError`; **entity didn't render for
  the stamped principal → `BuildError`** (the renderability gate, DR-S4).
- Fix the prototype `data-entry.yaml` dangling view ref (`ticket_report`→`ticket`).
- An example manual section using `screenshot{}` + docs.

OUT of scope:
- Video/GIF capture; multi-step interaction recordings (openvwr's badge-numbered
  flows) — arrow/box only for v1.
- Bundling a browser; `screenshot{}` requires a Chrome/Chromium already present.
- Running the browser-gated tests in the standard CI matrix (they gate on Chrome
  + a built SPA, like the existing e2e job) — covered by a dedicated tag/skip.

**Acceptance Criteria:**

1. **Capture a form** — `screenshot{ view="form", type="ticket", entity=<id>,
   out="f.png" }` produces `f.png` (a non-trivial PNG) and emits `![](f.png)`.
   (Browser-gated test: seed a ticket, capture, assert the file exists + is a
   valid PNG + the markdown ref is emitted.)
2. **Element clip** — `clip="[data-field=status]"` (or a field shorthand)
   captures just that field's bounding box, not the full page. (Test: assert the
   clipped image dimensions are smaller than full-page.)
3. **Arrow annotation** — `arrows={{at="status", text="auto-computed"}}` draws an
   arrow+label anchored to `[data-field=status]` before capture. (Test: the
   overlay element exists in the DOM at capture time; a golden-ish assertion that
   the annotated region differs from unannotated.)
4. **Role scoping** — `as="viewer"` maps a per-navigation role header to a viewer
   principal; the harness asserts the differential set of `#field-<prop>` present
   for editor vs viewer (the renderability probe doubles as this test, DR-L2).
   (Browser-gated test: editor sees fields viewer doesn't.)
5. **Fail-loud: no browser** — when no Chrome binary resolves, `screenshot{}`
   returns a `BuildError` naming the manual line (NOT a placeholder, NOT a
   silent skip). (Test: run with an env that hides Chrome → BuildError.)
6. **Fail-loud: unknown anchor** — `at="nosuchfield"` → `BuildError` naming the
   field + manual line. (Test.)
7. **Fail-loud: SPA not built** — if `static/v2/index.html` is absent,
   `BuildError` with a clear "run just build-frontend" hint. (Test via
   `CheckEmbeddedSPA` seam.)
8. **Renderability gate (DR-S4)** — if the entity fails to render for the stamped
   principal (the SPA shows its error boundary, `data-testid` present, or a known
   field's `#field-<prop>` never appears), `screenshot{}` returns a `BuildError`
   — it never embeds a PNG of an error state. (Browser-gated test: seed an entity
   the stamped role can't read → BuildError, not a broken PNG.)
9. **Annotation injection-safety (DR-C2)** — arrow text containing `"`,
   `</script>`, U+2028/2029 is json.Marshal-escaped into the overlay JS and cannot
   break out. (Unit test on the JS generator, no browser needed.)
10. **Non-screenshot manuals unaffected** — a manual with no `screenshot{}` builds
    with no browser/SPA dependency touched (the harness is lazy — only a
    `screenshot{}` island triggers server+browser standup). (Test: a Tier-A-only
    manual builds fine with Chrome hidden; the Capturer is never constructed.)
11. **Example manual** — a committed manual section renders a real annotated
    form screenshot end-to-end (browser-gated integration test).

## Research

- [x] ~~/research~~ (RES-EK7LSA addendum "Tier B / phase 3" + openvwr study cover this)
- [x] Searched for existing libraries — chromedp already in tree (v0.16.0)
- [x] Checked codebase for similar patterns — `internal/dataentry/e2e_test.go` (chromedp), `appbuild.Discover` (wiring)
- [x] **Spiked the end-to-end path** — proven working (see Understanding)

**Research Doc:** RES-EK7LSA (Tier B section). **Spike:** captured a real form PNG this session.

**Existing Solutions (grounded, file:line):**
- **chromedp v0.16.0** (`go.mod:11`) — `NewExecAllocator`/`NewContext`/`Run`,
  verbs `Navigate`/`WaitVisible`(ByQuery)/`Sleep`/`FullScreenshot`/`Screenshot`
  (element clip)/`Evaluate` (for the annotation overlay). Pattern in
  `internal/dataentry/e2e_test.go:97-126` (NOTE: that file is stale — 9-arg
  `NewApp`; not in CI. Reference only.)
- **Server wiring** — `appbuild.Discover(projectDir, script.NewEngine())`
  (`appbuild.go:528`) → `*Services` getters → `dataentry.NewApp(...)`
  (`app.go:339`, 10 args incl. `svc.VisibleSearcher()`). `app.NewRouter()`
  (`router.go:49`) serves the embedded SPA (`static.go:7`, `//go:embed static/*`).
  `dataentry.CheckEmbeddedSPA()` (`router.go:33`) guards the build prereq.
- **Principal stamping** — `app.SetPrincipalResolver(func(*http.Request)
  principal.Principal)` (`app.go:314`), before `NewRouter`. Maps a request to a
  role's principal for `as=`.
- **SPA routes** (`frontend/src/router/index.ts`): edit form `/form/<formId>/<entityId>`,
  create `/form/<formId>`, entity `/entity/<type>/<id>`, list `/list/<listId>`.
- **Field anchor** — add `:data-field="field.property"` to `FieldShell.vue`'s
  `.form-field` root (`frontend/src/components/forms/FieldShell.vue:16`).
  `FieldRenderer` already computes `field-${property}` (`FieldRenderer.vue:41`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Package placement (resolves open-Q6) — a SEPARATE package `internal/docscapture`.**
The core `internal/docs` must stay browser-free and light (Tier A runs anywhere).
`screenshot{}` needs `dataentry` + `appbuild` + `chromedp` — a huge dep surface.
So: `internal/docscapture` owns the harness (server standup + chromedp + PNG);
`internal/docs` depends on it only through a **narrow consumer-side interface**
(CLAUDE.md rule) — e.g. `type Capturer interface { Capture(ctx, CaptureSpec)
(imagePath string, err error) }`. `internal/docs` defines the interface; the CLI
wiring injects the `docscapture` implementation. A manual with no `screenshot{}`
never calls it, so Tier-A builds pull in nothing new. If the Capturer is nil (not
wired) and a manual uses `screenshot{}` → BuildError.

**The seed-to-temp-project bridge (revised per DR-S2).** A single
`applySeed(store.Store)` function replays the manual's `create`/`link` calls.
For Tier-A resolvers it runs against the in-mem memstore (phase 2, unchanged).
On the first `screenshot{}`, the Capturer stands up a temp project = copy the
documented project's schema/config files (`metamodel.yaml`, `data-entry.yaml`,
`acl.yaml`, `templates/`) into a temp dir, `Discover` it, then **run the SAME
`applySeed` against the temp project's `store.Store`** (raw `CreateEntity`, no
entitymanager → no automations mutating the fixture). fsstore owns its own
on-disk format; we never hand-write markdown. One seed function → the two stores
cannot diverge. The server is stood up ONCE per build (lazy), reused across
islands, and its **lifecycle lives on the Capturer** (owns temp dir + server);
`Build` `defer`s `capturer.Close()` unconditionally so a panic can't leak them
(DR-M3). The `as=` role is threaded per-navigation via a request header the
installed resolver maps (DR-S1).

**Implementation order (hard-first):**
1. **`internal/docscapture`** — the harness, proven-recipe from the spike:
   temp-project standup (copy schema; seed via `applySeed` against the temp store),
   `Discover`+`NewApp`+ a **request-inspecting** `SetPrincipalResolver` (header→role→
   principal), `http.Server` on a free `127.0.0.1` port, chromedp allocator/context,
   navigate (with the role header) + `WaitVisible` + capture (full or element-clip).
   Chrome discovery via `exec.LookPath` over the known names → BuildError if none.
   The Capturer owns the temp-dir + server lifecycle with a `Close()`.
2. **Renderability gate + annotation overlay** — after navigate, assert the entity
   rendered (a known `#field-<prop>` visible AND the error-boundary `data-testid`
   absent) → BuildError if broken (DR-S4). Then inject the overlay JS (arrows/boxes
   anchored to `#field-<prop>`/ARIA) built by **`json.Marshal`-ing operator text
   into a JS string literal** spliced into `chromedp.Evaluate` (DR-C2). An anchor
   resolving to nothing → BuildError.
3. **`screenshot{}` resolver** in `internal/docs` (`doc.*`, via the Capturer
   interface): parse args, call the Capturer, write the PNG beside the output
   (relative path), emit `![alt](rel/path.png)`.
4. **CLI wiring** — inject the `docscapture.Capturer` into the doc build in
   `internal/cli/docs.go`; the PNG out-dir derives from `--out`.
5. **SPA edit** — add a stable `data-testid` to the "entity failed to load" error
   boundary component (the ONLY SPA change; rebuild `static/v2`). Anchoring uses
   the EXISTING `#field-<prop>` id — no field-hook edit needed (DR-C1).
6. Fix the prototype `data-entry.yaml` view ref; example manual + docs.

**Files to add/modify:**
- ADD `internal/docscapture/{capture.go, server.go, annotate.go, *_test.go}`.
- ADD `internal/docs/screenshot.go` (the `doc.screenshot` binding + `Capturer`
  consumer-side interface defined HERE).
- EDIT `internal/docs/module.go` (register `screenshot`), `runtime.go` (hold the
  injected `Capturer`, `defer Close()`), `internal/cli/docs.go` (inject the impl).
- EDIT the SPA error-boundary component (add `data-testid`; rebuild `static/v2`).
- EDIT `.go-arch-lint.yml`: add `docscapture` component; its `mayDependOn`
  (dataentry, appbuild, project, principal, acl, storage, docs? NO — docs must not
  be a dep of docscapture either; the interface is in docs but docscapture defines
  its own `CaptureSpec` DTO); **register `chromedp` in the `vendors:` block** and
  `canUse: chromedp` for docscapture (DR-S5). Assert `docs`'s block is UNCHANGED.
- EDIT `prototypes/data-entry/project/data-entry.yaml` (view-ref fix).
- ADD example manual section + guide update.

**Alternatives considered:**
- *Claude-in-Chrome MCP for capture* — rejected: session-tooling, not committable
  or CI-runnable.
- *Playwright (Node)* — rejected: new Node/browser toolchain; chromedp is already
  in-tree and Go-native.
- *Hand-wired memstore (no bleve)* — rejected: pointless complexity next to Chrome;
  `Discover` over a temp project is simpler and uses real wiring.
- *DOM-overlay vs post-capture Go image compositing for annotations* — chose DOM
  overlay (anchors to live element geometry the browser already computed; openvwr's
  approach). Go compositing would re-derive coordinates. Revisit only if overlay
  proves flaky.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- The manual + islands are operator-authored (same trust as phase 2). `screenshot{}`
  args (view/type/entity/field names) validate against the metamodel + seeded set;
  unknown → fail-loud.
- **Annotation text** is operator-authored and injected into a DOM overlay.
  `chromedp.Evaluate` takes ONLY a JS expression string (no args channel — DR-C2),
  so the safe technique is `json.Marshal(text)` → the result is a valid, fully-
  escaped JS string literal (handles `"`, `</script>`, `\`, U+2028/2029) → splice
  THAT literal into the expression. NEVER raw string-concatenate operator text.
  AC9 tests hostile text against this path.
- The temp project is written under `os.MkdirTemp`; only schema files + the
  manual's own seed entities are copied in. Torn down at build end.
- The capture server binds `127.0.0.1` on an ephemeral port, no external exposure.

**Security-Sensitive Operations:**
- Headless Chrome launch — try WITH the sandbox first; only add `--no-sandbox` as
  a fallback when the sandboxed launch fails (e.g. root/container), rather than
  unconditionally like the e2e test (DR-M1). Page content is our own seeded
  fixture on localhost, so the tradeoff is low-risk, but default-secure is better.
- The capture server runs with a real Declarative ACL but a build-chosen stamped
  principal (`as=`); it only ever serves the ephemeral seed, never real data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**
- **Browser-gated integration tests** (`internal/docscapture`, a `//go:build` tag
  or `exec.LookPath("chrome")`-skip): stand up the temp-project server, capture a
  seeded ticket form, assert PNG validity + dimensions (full vs clip) + the
  annotation overlay presence. Model on `e2e_test.go`; SKIP (not fail) when Chrome
  or `static/v2` is absent, so the standard CI matrix stays green; a dedicated
  browser CI job (or the existing e2e job) runs them.
- **Non-browser unit tests** (always run): arg parsing, the seed-materialization
  (temp project has the right files + seed markdown), the annotation-JS generator
  (injection-safe: a `"`/`</script>` in text can't break out), the Capturer-nil →
  BuildError path, the SPA-absent → BuildError path (mock the CheckEmbeddedSPA seam).
- **Frontend unit test** — FieldShell renders `[data-field]` (AC8).
- **Fail-loud** — no-Chrome, unknown-field, SPA-absent each assert a BuildError
  with the manual line (AC5/6/7), testable WITHOUT a browser by faulting earlier.

**Edge Cases:**
- Manual with no `screenshot{}` → Capturer never invoked, no browser/SPA needed (AC9).
- Multiple `screenshot{}` islands → server stood up once, reused.
- `entity` id not in the seed → fail-loud.
- A field that exists in the metamodel but not on the chosen form (data-entry.yaml
  omits it) → the `[data-field]` anchor won't be in the DOM → fail-loud (correct;
  the annotation would be meaningless).
- Very tall form → full-page screenshot height cap (DR-M2): specify a max height
  (e.g. 4000px); exceeding it is a **BuildError** (loud), not a silent truncation
  — consistent with the fail-loud identity.
- Chrome present but crashes/times out → BuildError (wrap the chromedp ctx deadline).

**Negative Tests:** no-Chrome, unknown field, unknown entity, unknown view,
SPA-not-built, Capturer-not-wired — each a BuildError.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- **Biggest: browser-gated tests + core capture/geometry logic under-covered in
  standard CI (DR-S3).** Mitigation: (a) unit-test everything non-browser (arg
  parsing, `applySeed`, the annotation-JS generator incl. injection, the JSON-
  literal escaping, all fail-loud paths incl. nil-Capturer/SPA-absent by faulting
  before the browser); (b) make the browser-gated job **required on PRs touching
  `internal/docscapture`** (path filter) — not green-by-skip — so a chromedp
  regression can't merge silently; (c) split annotation GEOMETRY so the
  coordinate math is testable against a static HTML fixture, not only a full
  capture. The FEATURE fails loud without a browser — that's intended; the TESTS
  must not silently pass by skipping.
- **SPA build coupling.** `screenshot{}` needs `static/v2` built. Mitigation:
  `CheckEmbeddedSPA` fail-loud with the `just build-frontend` hint; documented as
  a prerequisite. Tier-A unaffected.
- **chromedp flakiness / timing.** The spike needed a settle `Sleep`. Mitigation:
  `WaitVisible` on a stable anchor + a bounded settle; a per-capture timeout →
  BuildError, never a hang.
- **Annotation-overlay injection.** Mitigation: parametrized `Evaluate`, never
  string-spliced JS; test with hostile text.
- **arch-lint dep surface.** `docs`→`docscapture` must be via a narrow interface
  only (no direct dataentry/chromedp import from `docs`). Mitigation: consumer-side
  `Capturer` interface in `docs`; impl in `docscapture`; run arch-lint early.
- **Effort: l** (new harness package + resolver + annotation overlay + SPA hook +
  browser-gated tests + example). The spike de-risked the core, so it's a
  well-scoped l, not xl.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/rela-docs.md` (source `GUIDE-rela-docs.md`) — add the `screenshot{}`
  section: args, anchoring, `as=` roles, the Chrome + built-SPA prerequisites,
  fail-loud behavior. Regenerate via `just docs`.
- [x] CLAUDE.md — a short face to `internal/docscapture` (new browser-dep
  subsystem, kept off core `internal/docs`).
- [x] The example manual section doubles as living docs.
- [x] ~~docs/data-entry.md~~ (the `data-field` hook is an internal SPA test-hook;
  mention only in FieldShell godoc/comment).

## Design Review

- [x] Run `/design-review` before starting implementation (cranky reviewer, verified against real code + chromedp/cdproto source)
- [x] All critical/significant findings addressed in plan

**Design Review Findings** (all addressed in-plan before implementation):

- **DR-C1 (critical) — `data-field` on FieldShell unbuildable + "no `#field-<prop>`
  hook" was WRONG.** FieldShell has no `field.property`; but `FieldRenderer.vue:41/88`
  DOES stamp `#field-<prop>` on the widget input. RESOLVED: anchor on the existing
  `#field-<prop>` id — **zero SPA field-hook edit**. (A `dataField` prop on
  FieldShell for the wider box is deferred.)
- **DR-C2 (critical) — `chromedp.Evaluate` has no args channel;** the "parametrized
  Evaluate" safety mechanism doesn't exist. RESOLVED: `json.Marshal(text)` → a
  fully-escaped JS string literal spliced into the expression; AC9 tests hostile
  text (`"`/`</script>`/U+2028/2029).
- **DR-S1 (significant) — `as=` fixed-principal vs. reused server contradiction.**
  RESOLVED: the resolver is per-request (router.go:110/287); install a
  request-inspecting resolver + a per-navigation role header. One server, correct
  per-island roles.
- **DR-S2 (significant) — two divergent seed representations.** RESOLVED: one
  `applySeed(store.Store)` replays `create`/`link` against BOTH the in-mem store
  and the fsstore temp store (raw, no automations); no hand-written markdown, so
  they can't drift.
- **DR-S3 (significant) — core capture logic untested in standard CI.** RESOLVED:
  browser job REQUIRED on `internal/docscapture` PRs (not green-by-skip); split
  annotation geometry to be testable against a static fixture.
- **DR-S4 (significant) — fail-OPEN: capturing a broken "entity not found" form**
  (the spike hit this). RESOLVED: a renderability gate (known `#field-<prop>`
  visible + error-boundary `data-testid` absent) → BuildError; AC8 covers it. The
  gate doubles as the AC4 role-differential probe (DR-L2).
- **DR-S5 (significant) — chromedp not in arch-lint `vendors:`.** RESOLVED: register
  it + `canUse: chromedp` on `docscapture`; assert `docs`'s block unchanged.
- **DR-M1 no-sandbox** → try-sandboxed-first, fallback. **DR-M2 tall-form** →
  height cap = BuildError. **DR-M3 lifecycle** → on the Capturer, `Build` defers
  `Close()`. All folded in above.
- **DR-L1/L2 (leverage)** — noted: could collapse to one fsstore for
  screenshot-bearing builds (deferred; the applySeed unification already removes
  the divergence risk); the renderability probe = the role-differential test.
