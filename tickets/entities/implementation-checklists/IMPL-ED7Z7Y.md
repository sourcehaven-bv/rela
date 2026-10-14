---
id: IMPL-ED7Z7Y
type: implementation-checklist
title: 'Implementation: Lua rela.xml.decode and rela.feed.parse with hardened XML limits'
started: "2026-10-13"
completed: "2026-10-14"
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

**Verification Evidence:** `rela script` (local build) fetching live feeds with
`http.request` and parsing with `rela.feed.parse`:
- go.dev/blog/feed.atom → atom, 10 items, tag: id, published 2026-10-02T00:00:00Z, content converted by `rela.md.from_html` (17 KB).
- hnrss.org/frontpage → rss, 20 items, link fallback id, RFC 1123 date → UTC.
- jsonfeed.org/feed.json → json, 2 items.
- `rela.xml.decode` + `find` + `{uri}name` attr key + `text` on a namespaced doc: "v", "tu".
Hostile input (billion laughs, XXE, depth, element/attribute/byte caps,
charsets, 10 001 items in RSS/Atom/JSON, cyclic Lua tables) covered by
`internal/safexml`, `internal/feed` and `internal/lua/xml_test.go`; all pass
under -race. safexml coverage 100%.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — markup serialization shared by RSS and Atom (markup.go); one decode-error helper for both Lua bindings
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
