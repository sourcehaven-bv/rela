---
id: PLAN-0GCF
type: planning-checklist
title: 'Planning: Drop user-visible /v2/ URL prefix and remove stale HTMX app.js'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

User-visible cleanup of `/v2/` URL prefix and removal of leftover HTMX `app.js`.
Per user clarification, the goal is "the stuff a user sees" — server-internal
naming (`/api/v1/`, `handleV1*`, `api_v1.go`, `static/v2/` build output dir) is
explicitly OUT of scope because there is no harm in keeping them.

**In scope:**
- `frontend/vite.config.ts` and `frontend/vite.config.js`: change `base` from `'/v2/'` to `'/'` for production builds
- `frontend/index.html`: change favicon href from `/v2/favicon.svg` to `/favicon.svg`
- `frontend/src/router/index.ts`: update comment (BASE_URL is now `'/'` everywhere; logic stays unchanged because `import.meta.env.BASE_URL` still returns the new base)
- `internal/dataentry/router.go`: remove the `mux.Handle("/v2/", ...)` legacy alias and the "backward compatibility" comment; rename `v2FS` local variable to `spaFS` for clarity; update panic message
- `internal/dataentry/middleware_security_test.go:78`: drop `/v2/` from the exempt-path test list (the path no longer exists)
- Delete `internal/dataentry/static/app.js` (74KB HTMX leftover, embedded but referenced from no Go or Vue code)
- Delete `tickets/entities/automated-measures/codemirror-textarea-sync.md` (automated-measure pinned to `app.js`, describes HTMX form-submission behaviour that no longer exists)

**Out of scope (explicitly):**
- `/api/v1/` REST API path — server-only, no harm in keeping
- `internal/dataentry/static/v2/` build output directory name — gitignored, never seen by users
- Internal Go naming (`api_v1.go`, `handleV1*`, `registerAPIV1Routes`) — server-only
- `/api/v1/_events` SSE alias — server-only
- `frontend/vite.config.js` / `frontend/vite.config.d.ts` being checked into git despite being TS-compiled artifacts — pre-existing issue, not this ticket's concern (will update them in lockstep with the .ts file)

**Acceptance Criteria:**

1. **No `/v2/` in browser address bar.** Build the SPA, run `rela-server`, navigate the app — every URL in the browser is `http://localhost:8080/...` with no `/v2/` segment.
   - Test: build with `just build-frontend && just build-server`, run server, manually verify `/`, `/list/ticket`, `/entity/ticket/TKT-001`, `/settings`, `/graph`, `/search`, `/kanban/...` all render correctly.
2. **Favicon loads at root.** Browser dev-tools network tab shows favicon fetched from `/favicon.svg`, returns 200.
   - Test: open built app, check Network tab.
3. **No regression in SPA deep-link refresh.** Refreshing on a deep-linked route (e.g. `/list/ticket`) still loads the SPA correctly (the catch-all `mux.Handle("/", spaHandler(spaFS))` must still serve `index.html` for unknown paths).
   - Test: navigate to `/list/ticket`, refresh, verify list renders.
4. **Embedded `app.js` is gone.** `strings bin/rela-server | grep -c "EasyMDE"` returns 0; binary size is reduced by ~74KB.
   - Test: build server, inspect binary.
5. **`codemirror-textarea-sync` automated-measure is removed.** `analyze_orphans` and `analyze_validations` still pass after the deletion.
6. **All Go tests, frontend unit tests, e2e tests, and lint pass.** `just test`, `just lint`, `cd frontend && npm run test:run`, `cd frontend && npm run test:e2e` all green.
7. **Desktop app still works.** `just build-desktop` produces a binary that loads the SPA without 404s on assets (BUG-W144 was about desktop missing the SPA dir entirely; this ticket only changes the URL base, not the dir, so should be safe).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- N/A — pure cleanup, no new functionality. Vite's `base` option is the standard mechanism for SPA URL prefixing and is already in use.
- Prior work: `TKT-tv5u` ("Remove v1 HTMX UI code after Vue migration") removed templates and most static assets but left the `/v2/` URL prefix and `app.js` behind. This ticket finishes that cleanup.
- Related concept: `data-entry-ui` (still describes HTMX in its summary — could be updated, but that's a separate documentation cleanup, out of scope here).
- Codebase grep for `/v2/`, `static/v2`, `BASE_URL`, `app.js` confirms the file inventory below is complete.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

This is a mechanical refactor in 4 small steps. No behaviour change beyond URL
routing.

**Step 1 — Frontend build base:**
- `frontend/vite.config.ts:16`: change `base: command === 'build' ? '/v2/' : '/'` → `base: '/'`
- `frontend/vite.config.js:14`: same change in the compiled .js file
- `frontend/vite.config.ts:15`: drop the now-stale comment about "Use /v2/ base in production build"
- `frontend/index.html:5`: change `href="/v2/favicon.svg"` → `href="/favicon.svg"`
- `frontend/src/router/index.ts:84`: update comment to "Use Vite's BASE_URL ('/' for both dev and prod)" — code stays the same because `createWebHistory(import.meta.env.BASE_URL)` continues to work correctly with the new base.

**Step 2 — Backend router:**
- `internal/dataentry/router.go`:
  - Drop `mux.Handle("/v2/", http.StripPrefix("/v2/", spaHandler(v2FS)))` (line 26)
  - Drop the comment "Vue SPA - serve at root and /v2/ for backward compatibility" (line 21)
  - Rename local variable `v2FS` → `spaFS` for clarity (the directory still happens to be `static/v2` on disk, but that's an internal detail)
  - Update panic message from `"embedded v2 filesystem: "` → `"embedded SPA filesystem: "`
  - The `fs.Sub(staticFiles, "static/v2")` call stays unchanged — Vite still outputs there; no need to rename the gitignored build dir
- `internal/dataentry/middleware_security_test.go:78`: drop `"/v2/"` from the test slice; the path no longer exists so testing it would be misleading

**Step 3 — Delete leftover HTMX app.js:**
- `rm internal/dataentry/static/app.js`
- Verify `go build ./...` still works (the file is embedded via `//go:embed all:static/*` in `static.go`, which uses a glob — removal is automatic, no Go code change needed)

**Step 4 — Delete stale automated-measure:**
- `rm tickets/entities/automated-measures/codemirror-textarea-sync.md`
- This measure tracks "EasyMDE/CodeMirror editor syncs content to textarea on changes for HTMX form submissions" — there are no HTMX form submissions any more. The Vue app uses its own `MarkdownEditor` component with reactive bindings. The measure has no analogue worth keeping.
- Run `analyze_orphans` to confirm nothing else points to it.

**Files to modify:**

| File | Change |
|------|--------|
| `frontend/vite.config.ts` | `base: '/'`, drop comment |
| `frontend/vite.config.js` | same (lockstep with .ts) |
| `frontend/index.html` | favicon href → `/favicon.svg` |
| `frontend/src/router/index.ts` | comment update only |
| `internal/dataentry/router.go` | drop `/v2/` alias, rename var, drop comment |
| `internal/dataentry/middleware_security_test.go` | drop `/v2/` from exempt list |
| `internal/dataentry/static/app.js` | DELETE |
| `tickets/entities/automated-measures/codemirror-textarea-sync.md` | DELETE |

**Alternatives considered:**

1. **Rename `static/v2/` build dir to `static/spa/`.** Rejected: gitignored, never user-visible, would touch `.gitignore`, both vite configs, `frontend/CLAUDE.md`, `router.go`, and the description text in `build-desktop-frontend-dep` automated-measure. The user explicitly scoped this to user-visible changes.
2. **Also rename `v2FS` local variable in router.go.** Accepted (renaming to `spaFS`) — it's a 1-line change, improves readability of code that any contributor will see, and aligns with dropping the `/v2/` HTTP route.
3. **Keep `/v2/` as a redirect to `/` for old bookmarks.** Rejected: `rela-server` is a local development tool, no users have stable bookmarks worth preserving; clean break is simpler.
4. **Delete `frontend/vite.config.js` (the compiled artifact).** Tempting, but pre-existing tracked file, separate cleanup, not this ticket's job.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- N/A — no input handling changes. The change reduces HTTP surface area (one route removed), it doesn't add any.

**Security-Sensitive Operations:**

- The `requireSameOrigin` middleware uses an opt-in `sensitivePathPrefixes` list, so removing `/v2/` from the **exempt** test does not change which paths are protected. Sensitive paths (`/api/`) are protected before and after.
- The catch-all `mux.Handle("/", spaHandler(spaFS))` still serves `index.html` for unknown paths — this is the Vue SPA shell with no secrets, so spa-routing fall-through is safe.
- Removing `/v2/` reduces, never expands, the attack surface.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | How tested |
|----|------------|
| 1. No `/v2/` in browser | Manual: build + run + click around. e2e suite already navigates with relative URLs so it implicitly verifies the base works. |
| 2. Favicon at `/favicon.svg` | Manual network tab; existing security tests cover `/static/` mount. |
| 3. Deep-link refresh works | Manual + existing e2e tests refresh pages after navigating |
| 4. `app.js` not embedded | `strings bin/rela-server \| grep -c EasyMDE` should be 0; `ls -la bin/rela-server` before/after for size diff |
| 5. Stale measure removed | `analyze_orphans` + `analyze_validations` after deletion |
| 6. All tests/lint pass | `just lint && just test && (cd frontend && npm run test:run && npm run test:e2e)` |
| 7. Desktop binary works | `just build-desktop` then run, pick a project, verify SPA loads |

**Edge Cases:**

- **Browser cache for `/v2/index.html`.** A user with the old build cached might hit a 404. Acceptable: this is a local dev tool, hard refresh resolves it. No production impact.
- **Vite dev server.** Already used `base: '/'` in dev mode, so dev workflow is unchanged.
- **Existing e2e tests with hardcoded `/v2/` paths.** Grep confirmed there are none in `frontend/e2e/`.
- **`createWebHistory(import.meta.env.BASE_URL)`.** With the new `base: '/'`, `BASE_URL` becomes `'/'` in both dev and prod. `createWebHistory('/')` is the standard Vue Router setup; no change in behaviour.
- **Embedded filesystem after removing `app.js`.** `//go:embed all:static/*` glob will simply not pick it up; no Go code change needed. `staticFiles.ReadFile("static/app.js")` would now error, but no code does that read.

**Negative Tests:**

- Existing security tests cover cross-origin requests to `/api/` — they should still pass unchanged.
- After deletion, `grep -rn "static/app.js" .` (excluding tickets/historical docs) should return zero matches in active code.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| Hardcoded `/v2/` somewhere in Vue source not caught by grep | Low | Medium | Manual click-through after build; e2e suite catches navigation issues |
| `import.meta.env.BASE_URL` consumed elsewhere with assumptions | Low | Low | Grep already done — only `router/index.ts` references it |
| `app.js` referenced from a doc/tutorial that breaks | None | Low | Only references are in `vue-migration-plan.md` (historical) and in tickets/review-responses (historical records); nothing executable |
| Desktop app regression | Low | Medium | BUG-W144 was about empty `static/v2` dir; this ticket doesn't empty it, just changes URL routing; verify with `just build-desktop` |
| Stale `frontend/vite.config.js` and `.ts` drift | Already exists | Low | Update both in lockstep |

**Effort:** s (small) — confirmed. Maybe 30–60 minutes of work + verification.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — Internal refactor with no externally-documented contract change. The `/v2/` URL was never documented as an API for users; it was an implementation detail of the Vite build. `data-entry-ui` concept's HTMX-flavoured description is pre-existing and out of scope (separate cleanup ticket if desired).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: skipped for this mechanical refactor after explicit user approval — "continue". Scope is small, pure deletion + config flip, no architectural decisions to review.)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** None — cranky-code-reviewer agent was run at the `review` phase instead, and caught a significant miss (RR-EEK5, repo-root e2e suite). Findings recorded in REV-O5RA.
