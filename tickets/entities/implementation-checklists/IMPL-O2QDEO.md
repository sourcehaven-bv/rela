---
id: IMPL-O2QDEO
type: implementation-checklist
title: 'Implementation: CalDAV go-webdav adapter, VTODO collections, getctag + two-way PUT/DELETE'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The adapter is
`internal/dataentry/caldav_{backend,write,ical,mapping,ctag,handler}.go`, with
`go-webdav` vendored behind an arch-lint grant. `*ical.Calendar` does not escape
the adapter.

Integration is covered at two levels: `doCalDAV` drives the FULL router in tests
(so the middleware chain, ACL gate and JWT/host checks all run), and the
`.ignored/caldav-demo` project ran against a real Apple Reminders account
through Pratique with TLS.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Shared `caldavTestApp` / `doCalDAV` helpers; table-driven subtests throughout.

**Test-quality lesson from the review, worth recording.** The unit tests were
thorough on *individual* operations and blind to *sequences* — which is where a
sync protocol actually lives. Four of the five critical defects were only
reachable by composing operations (PUT → DELETE → replay) or by mixing in an
edit made outside CalDAV. Every regression test added for those was verified to
FAIL against a simulated reintroduction of the bug before being kept.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Live against Apple Reminders (macOS) via Pratique over TLS, plus direct
`curl`/PROPFIND probes against the demo server. The per-AC table is in the
review checklist **REV-E7QYNN**; AC9 (Thunderbird / eM Client / Cfait) is
explicitly **not met**.

The live demo found bugs the green test suite did not, repeatedly — CSRF
blocking every CalDAV request, `/.well-known/caldav` returning SPA HTML, the
site-root principal probe, YAML dates decoding to `time.Time`, and the
`where:`-hides-completed footgun. All shared one shape: a test that constructs
the designed-for request cannot catch what a real client does *before* it.

Edge cases verified live: out-of-band deletion (`rm`, simulating `git pull`),
replay of a cached PUT after delete, conditional writes both fresh and stale, a
corrupt entity (retryable vs deleted), and a corrupt alias table.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Reads route through the same ACL-scoped `feedEntitySource` the ICS feed uses;
writes go through `entitymanager.PatchEntity` naming only changed properties, so
unmapped and redacted properties survive. The reviewer independently confirmed
both (no ctag existence-oracle between principals; `secret` survives a client
write).

`renderObject` is the single place an entity becomes a CalDAV resource, so the
ETag a PUT returns is computed over exactly what a later GET serves — the rule
whose violation caused RR-6P8QL8.

`just lint` 0 issues, `just arch-lint` OK, `just plimsoll` no new offenders.
