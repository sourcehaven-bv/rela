---
id: PLAN-PXBRN
type: planning-checklist
title: 'Planning: Remove dead htmx templates and vendor-js justfile target after Vue migration'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
1. Delete `internal/dataentry/templates/form.html` (368 lines) and `internal/dataentry/templates/_partials.html` (21 lines), then remove the empty `internal/dataentry/templates/` directory.
2. Remove the `vendor-js` target from `justfile` (lines 230–243). It downloads `htmx.min.js` and 7 other libs into `internal/dataentry/static/`. None of those files are checked in or embedded; the Vue frontend bundles them via `frontend/package.json`.
3. Update the `data-entry-ui` concept in `tickets/entities/concepts/data-entry-ui.md`: replace the htmx/Go-templates description and "HTMX-powered web interface" summary with the current Vue SPA / Pinia / Vite / npm-bundled architecture.

Out of scope:
- Vue SPA changes (no UI/UX behavior changes).
- Removing `internal/dataentry/static/favicon.svg` — still served by the `/static/` route in `router.go:39`.
- Renaming `static/v2` → `static` — handler still exposes `/static/v2/*`; rename is tracked in TKT-MNOO (not this ticket).
- Updating other rela ticket/feature/planning files that mention htmx in historical context — those are immutable history.
- Touching `frontend/package.json`.

**Acceptance Criteria:**

1. **AC-1 — Templates gone:** `internal/dataentry/templates/` directory does not exist. Verified by `ls internal/dataentry/templates/` returning "No such file or directory".
2. **AC-2 — vendor-js gone:** `justfile` has no `vendor-js` target. Verified by `grep -n vendor-js justfile` producing no output, and `just --list` not showing it.
3. **AC-3 — No new htmx references in live code/config:** `grep -rn "htmx\|hx-" internal/ frontend/src/ justfile cmd/` returns no hits in non-archival files.
4. **AC-4 — Concept updated:** `tickets/entities/concepts/data-entry-ui.md` description and summary describe the Vue SPA architecture (Vue 3, Pinia, Vite, etc.) — no mention of "HTMX" or "Go HTML templates" in present tense.
5. **AC-5 — Build still works:** `just build` succeeds. `just test` passes (race-enabled). `just lint` clean. `just arch-lint` clean. `just coverage-check` passes.
6. **AC-6 — Server still serves correctly:** Embedded SPA still loads (`CheckEmbeddedSPA` at `internal/dataentry/router.go:16` succeeds). `/static/favicon.svg` still resolves.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- N/A — this is dead-code removal, not a new feature.
- Verified `internal/dataentry/static.go` only embeds `static/*`, NOT `templates/*` (`internal/dataentry/static.go:7`). The template files are not compiled into any binary.
- Verified no `.go` source under `internal/dataentry/` parses or references `form.html` / `_partials.html` (`grep -rn "form.html|_partials" internal/dataentry/*.go` → empty).
- Verified `vendor-js` is not invoked in `.github/` workflows (`grep -rn "vendor-js" .github/` → empty). It's a manual developer convenience that no longer matches reality.
- Verified Vue frontend bundles the previously-vendored libs: `easymde`, `slim-select`, `mermaid`, `cytoscape` all present in `frontend/package.json`. (Tagify and EasyMDE replaced — see TagSelect/MarkdownEditor Vue components.)
- The `data-entry-ui` concept content is a documentation artifact that has drifted; the FEAT-24hp feature (Vue migration) is already `implemented` but the concept it `requires` was never updated.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Three independent file-system operations followed by a verification pass:

1. `git rm internal/dataentry/templates/form.html internal/dataentry/templates/_partials.html` — `git rm` removes the directory automatically when empty.
2. Edit `justfile`: remove the `# ── Vendor ──` heading, the `# Vendor JS/CSS dependencies (commit the results)` comment, and the entire `vendor-js:` recipe (lines ~230–243). Leave the next section (`# ── Icons ──`) intact.
3. Edit `tickets/entities/concepts/data-entry-ui.md`: rewrite the `description` frontmatter property and `summary` to describe the current Vue SPA. Keep `package: internal/dataentry` and `layer: server` (still accurate). Status remains `stable`.

Verification: run `just build`, `just test`, `just lint`, `just arch-lint`,
`just coverage-check`. Manually start `just dev` and confirm the SPA loads at
`/` and `/static/favicon.svg` still resolves.

**Alternatives considered:**

- **Keep `vendor-js` for offline development.** Rejected: the Vue build (`just build-frontend`) is the canonical asset pipeline; `vendor-js` writes to a directory that no Go code reads. Keeping it is misleading.
- **Mark templates as `// Deprecated` and leave them.** Rejected: they are not Go code, just orphaned HTML. Comment-deprecation has no effect.
- **Bundle this with the `static/v2` → `static` rename (TKT-MNOO).** Rejected: that's a separate change with its own risk surface (vite config, embed paths). This ticket should be a clean no-functional-change diff.

**Files to modify:**

| Path | Operation |
|------|-----------|
| `internal/dataentry/templates/form.html` | delete |
| `internal/dataentry/templates/_partials.html` | delete |
| `justfile` (lines ~230–243, `vendor-js:` target + heading comments) | edit |
| `tickets/entities/concepts/data-entry-ui.md` | edit (frontmatter `description`, `summary`) |

**Dependencies:** none. No package imports change, no Go interfaces change, no
schema migrations.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- N/A. The change deletes static files and removes a build-tool recipe. No runtime input handling is added or modified.

**Security-Sensitive Operations:**

- Removing `vendor-js` slightly *reduces* attack surface: that target downloads JS from `unpkg.com` and `cdnjs.cloudflare.com` over HTTPS without integrity hashes. Killing it removes that supply-chain hop.
- No auth, crypto, file-access, or trust-boundary code is touched.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Verification |
|----|-------------|
| AC-1 | `test ! -d internal/dataentry/templates` returns 0 |
| AC-2 | `grep -n vendor-js justfile` returns empty; `just --list` does not show `vendor-js` |
| AC-3 | `grep -rn "htmx\|hx-" internal/ frontend/src/ justfile cmd/ \| grep -v _test.go` returns empty |
| AC-4 | `head -20 tickets/entities/concepts/data-entry-ui.md` shows updated description; `analyze_validations` and `analyze_properties` for rela-issues-and-design-tickets pass |
| AC-5 | `just build && just test && just lint && just arch-lint && just coverage-check` all green |
| AC-6 | `just dev` then `curl -s http://localhost:8090/` returns the SPA index.html; `curl -sI http://localhost:8090/static/favicon.svg` returns 200 |

**Edge Cases:**

- Stale local copies of `htmx.min.js` etc. on developer machines: not our concern; these were gitignored or never committed (verified `git ls-files internal/dataentry/static/htmx*` returns empty).
- Goimports / gofmt artifacts: none — no Go files modified.
- `arch-lint` rules referencing `internal/dataentry/templates`: none — the package-import boundary check operates on Go imports.

**Negative Tests:**

- After deletion, `go build ./...` must still succeed (it can't reference the deleted files; if it did, that would be a hidden bug we want surfaced).
- `just dev` must not error on missing template files (it doesn't — `app.go` has no `template.ParseFS("templates/*")`).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| A hidden code path still references the templates and tests don't cover it | low | `go:embed all:static/*` only embeds `static/*`, so templates can't be loaded at runtime. Confirmed by grep. |
| External tooling (IDE, doc generator) references the templates | very low | None of the codebase's documented tools (`just`, `golangci-lint`, `arch-lint`) parse HTML in `internal/`. |
| Concept description edit triggers metamodel validation failure | low | Run `analyze_properties` and `analyze_validations` after the edit; both run as part of the workflow. |
| Diff churn touches the `_partials.html` "Legacy HTMX" comment that other tickets rely on as anchor | very low | The comment is only inside the file we're deleting; nothing greps for it. |

**Effort:** xs (single-session, ~30 min including verification).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — Internal change, no user-facing docs needed
- [x] Concept documentation in `data-entry-ui.md` updated as part of this ticket (in scope, AC-4)
- [x] ~~User guide / reference docs~~ (N/A: no user-facing change)
- [x] ~~CLI help text~~ (N/A: no CLI changes)
- [x] ~~CLAUDE.md~~ (N/A: no new patterns)
- [x] ~~README.md~~ (N/A: no project-level changes)
- [x] ~~API docs~~ (N/A: no API changes)

This is `kind: chore`, not `enhancement`, so a separate `docs-checklist` is not
required by metamodel.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: pure dead-code removal — no Go source modified, no behavior change, diff is self-evident; user explicitly approved skipping design review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: design review skipped, no findings to address)

**Design Review Findings:** None — design review skipped (see above).
