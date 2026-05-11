---
id: IMPL-L5OQ
type: implementation-checklist
title: 'Implementation: Resolve entity-ID code spans to titled links in data-entry views'
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

- **Built `rela-server` against the in-tree `tickets/` project** and queried
`GET /api/v1/_views/ticket/TKT-77JD4` (a ticket whose body uses a clean ``
`FEAT-Q767` `` code span). Response carried `mentions["FEAT-Q767"] = {type:
"feature", title: "Keyboard shortcuts and power-user navigation in data-entry
UI"} ` exactly as planned (AC 1 + 2).
- **Unit tests passing:**
  - Go `internal/dataentry/mentions_test.go ` — 9 cases covering known
short-ID, manual-ID (concept), unknown, multi-token, fenced/indented code
blocks, link-text fixtures, dedup across blobs, inaccessible target (`git-crypt
` reason), empty content, plus self-reference test.
  - Go `internal/dataentry/api_v1_test.go ` — two new view-response tests
(`TestV1Views_MentionsPopulated `, `TestV1Views_MentionsAbsentWhenNoRefs `)
asserting the `mentions ` field is populated for refs and omitted entirely (no
`"mentions" ` JSON key) when there are none.
  - TS `frontend/src/utils/markdown.test.ts ` — 11 new cases under
`refResolver (entity-ID code spans) ` covering happy path, manual IDs, unknown,
multi-token, fenced blocks, existing link text, XSS-y title (DOMParser-based
assertion confirms no live `<img> `/`onerror `), inaccessible with tooltip,
unreasoned fallback, resolver exceptions swallowed, mixed known/unknown spans,
no-resolver backwards compat.
- **Round-trip corpus regression cleared** — `TestMdCorpusRoundTrip ` was
flagged on `PLAN-V6BB.md ` because a markdown bullet started with `+ ` on a
continuation line; reformatted the bullet to keep the corpus round-trip a fixed
point.
- **Full Go suite:** `go test -race ./... ` — all packages green
(`internal/dataentry ` 3.2s; `internal/lua ` 1.9s).
- **Full frontend suite:** `npm run test:run ` — 662 tests across 38 files
pass (was 651 before, +11 new ref-resolver cases).
- **Frontend typecheck:** `npm run typecheck ` — clean.
- **Frontend lint:** `npm run lint ` — no new errors (pre-existing
warnings unchanged).
- **e2e typecheck/lint:** `npm --prefix e2e run typecheck && npm --prefix
e2e run lint ` — both clean. Lint forced the use of a page-object method, so
added `contentEntityRefLink ` / `clickContentEntityRef ` helpers to `EntityPage
`.
- **Coverage:** `just coverage-check ` — total 75.5%, all package floors
satisfied.
- **Go lint + arch lint:** `just lint `, `just arch-lint ` — both clean.
- **Build:** `just build ` — `rela `, `rela-server `, `rela-desktop ` all
build successfully.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
