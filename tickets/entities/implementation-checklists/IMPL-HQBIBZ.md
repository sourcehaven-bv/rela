---
id: IMPL-HQBIBZ
type: implementation-checklist
title: 'Implementation: Feed subscriptions: poll RSS/Atom/JSON Feed into entities'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (subscription 93%, ingestspec 100%, scheduler subscriptions_test.go)
- [x] Integration tests written (appbuild TestSubscription_EndToEnd: loader, scheduler, queue, write path, state; second run sends the ETag and writes nothing)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no identity, max_items, deleted import, config reset, 304, redirects, caps)
- [x] Error handling in place (errors surfaced, not swallowed): transient errors fail the run; permanent rejections are logged and counted

## Test Quality

- [x] Using fixture builders or factories for test data (feedServer, rss builder, articleSub, subscriptionsYAML)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (link asserted as feed.URL+path)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Built rela, created a demo project with an `article`
type and a subscription to https://go.dev/blog/feed.atom (every 1h, max_items
5). `rela validate` reported "subscriptions are valid". `rela scheduler` logged
"scheduled subscription", "subscription skipped older items dropped=5" and
"subscription polled items=10 created=5"; five ART-*.md files had title, guid,
absolute link, published date and the summary as markdown. Repeat polls (304, no
duplicates) and rejections are covered by TestSubscription_EndToEnd and the
subscription run tests.

## Quality

- [x] Code follows project patterns (check similar code): mapping shared with webhooks via ingestspec/ingest; scheduler job path mirrors tasks
- [x] Checked for DRY opportunities: webhook mapping validation moved to ingestspec.Mapping.Validate and reused
- [x] No security issues introduced: same-origin redirects, body cap, query-free errors, sanitized HTML, http(s) links only
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
