---
id: IMPL-3S74S
type: implementation-checklist
title: 'Implementation: Remove dead htmx templates and vendor-js justfile target after Vue migration'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new code; deleted dead HTML files and removed an unused justfile recipe)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: no new functionality)
- [x] Happy path implemented (deletions + edits applied as planned)
- [x] Edge cases from planning handled (no Go embed of `templates/`, no CI invocation of `vendor-js`, `/static/favicon.svg` still served — all verified)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no error-handling code modified)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests added or modified)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Test | Result |
|----|------|--------|
| AC-1 | `test ! -d internal/dataentry/templates` | PASS — directory does not exist (`git rm` removed both files; dir auto-removed) |
| AC-2 | `grep -n vendor-js justfile` | PASS — empty output |
| AC-3 | `grep -rn "htmx\|hx-" internal/ frontend/src/ justfile cmd/ \| grep -v _test.go` | PASS — empty output |
| AC-4 | `head -25 tickets/entities/concepts/data-entry-ui.md` shows new Vue/Pinia/Vite description; no "HTMX" or "Go HTML templates" in present tense | PASS |
| AC-5a | `just build` | PASS — frontend `vite build` and both Go binaries compile |
| AC-5b | `just test` | PASS — full race-enabled suite green; coverage unchanged |
| AC-5c | `just lint` | PASS — `0 issues.` |
| AC-5d | `just arch-lint` | PASS modulo pre-existing `.ignored/` notices (verified by `git stash` baseline run; not introduced by this change) |
| AC-5e | `just coverage-check` | PASS — 74.2% total, package thresholds met |
| AC-6 | Started `go run ./cmd/rela-server -project prototypes/data-entry/project -port 18080`. `curl /` → 200 (SPA index.html); `curl /static/favicon.svg` → 200; `curl /static/v2/favicon.svg` → 200. Server logged `loaded project entities=14 relations=32` and `file watcher started for live-reload`. | PASS |

## Quality

- [x] Code follows project patterns (no Go code changed; concept-file edit follows existing YAML frontmatter format used elsewhere in `tickets/entities/concepts/`)
- [x] No security issues introduced (removing `vendor-js` slightly reduces supply-chain surface — it fetched JS from `unpkg.com`/`cdnjs.cloudflare.com` without integrity hashes)
- [x] No silent failures (no error-handling code touched; build/test/lint all surface failures as before)
- [x] No debug code left behind (no Go code changed)
