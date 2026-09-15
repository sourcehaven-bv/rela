---
id: REV-5VW9X4
type: review-checklist
title: 'Review: Named query scopes declared per entity type in schema.yaml, referenced by data-entry views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` — all pass
- [x] `just lint` — clean
- [x] `just coverage-check` — pass
- [x] `just arch-lint` — pass
- [x] `just plimsoll` — pass
- [x] `just comment-lint` — pass
- [x] Frontend: `npm run test:run`, `npm run lint`, `vue-tsc --noEmit`

**Evidence:** every touched Go package green (dataentry, scopes, appbuild,
queryplan, metamodel, dataentryconfig, cli, mcp, lua, analysis, tracer).
golangci-lint 0 issues across all of them. Coverage gate explicit: "Package
coverage threshold (50%) satisfied: PASS / Total coverage threshold (65%)
satisfied: PASS / Total test coverage: 74.8%". arch-lint "OK - No warnings
found". plimsoll clean. commentlint "no unresolvable doc links across 14855
comments". Frontend: 2487 tests in 154 files pass, 0 lint errors, typecheck
clean.

Also run against real PostgreSQL 16: the full pgstore suite (90s) plus the AC11
EXPLAIN test.

`cmd/rela-desktop` fails to build on this machine because the Xcode license has
not been accepted (cgo/wails), which makes the repo-wide `just lint` and `just
plimsoll` targets exit non-zero. Confirmed environmental and unrelated.

## Code Review

- [x] `/code-review` run
- [x] All critical findings addressed
- [x] All significant findings addressed
- [x] Minor/nit findings addressed or deferred with reason

Two reviewers, `cranky-code-reviewer` and `rela-security-reviewer`, run in
parallel over the branch. Eight review responses recorded and linked:

| RR | Severity | Status |
| --- | --- | --- |
| RR-U95ITB BindRequest never called | critical | addressed |
| RR-KH41AC view-declared query_scope inert | critical | addressed |
| RR-O9O030 _position walked the default scope | significant | addressed |
| RR-KMV4TD nil cfg/meta returned unscoped | significant | addressed |
| RR-7WCTCM identity error reported as a 500 | minor | deferred |
| RR-XZWGQ3 per-request recompile | minor | deferred |
| RR-U4H8BQ identity conjuncts derive a dead index | minor | deferred |
| RR-R65P4I countIsZero applies the default | minor | wont-fix |

The two critical findings are the review's value: both reviewers independently
found that `BindRequest` was declared, implemented, adapted through two layers
and never called, so the ticket's headline `mijn:` case returned 500 on every
page. The code reviewer separately found that `query_scope:` on a view was never
read at request time, so named scopes were inert while the config validated and
even derived an index. Neither was visible from the passing suite; both are now
pinned by tests that fail when reverted.

Each deferral names what makes it more than a tidy-up: a seam decision neither
side may make alone, a pattern shared with NextActionMatchers that should change
for both call sites at once, and a change that must be request-bound or it
shares one principal's identity with another.

## Verification

- [x] Each acceptance criterion verified
- [x] Manual verification performed
- [x] No regressions introduced

Full per-AC evidence in IMPL-XDZC30, including the EXPLAIN plans for AC11 and
the mutation testing of every authoritative check. AC7 is split to TKT-LYLO6P
and recorded as such on the ticket, with the reason it was worth separating.

## Documentation

- [x] Code documentation updated
- [x] Project documentation updated

`docs/metamodel.md` gains a "Query Scopes" section and `docs/data-entry.md`
gains `query_scope` in the list and kanban field tables plus a section on
composition with static filters and the `?query_scope=` API shape. Both written
in `docs-project/` and regenerated via `scripts/generate-docs.sh`.

One stale doc comment was corrected during review: it asserted in the present
tense that the SPA attached the parameter, describing code that did not exist.
It now says the client is a required participant and why.
