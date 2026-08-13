---
id: IMPL-3SIYV4
type: implementation-checklist
title: 'Implementation: CalDAV prep: VTODO renderer + completion fields in internal/calfeed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A:
`calfeed` is a leaf model→bytes package with no collaborators; the full flow is
the CalDAV protocol surface in TKT-MF1CWZ)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Note on the Apple-fixture tests: assertions there compare *rendered output
against the fixture's own parsed value* (`fixtureProp(t, ours, prop)` vs
`fixtureProp(t, fixture, prop)`) rather than against literals, so the fixture
stays the single source of truth.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The renderer was developed against **byte-exact captures from Apple Reminders**
(macOS 26.5.1, 2026-08-09) taken from a live Radicale sync, now committed as
`internal/calfeed/testdata/*.ics`. Per-AC status:

| AC | Status | Evidence |
|----|--------|----------|
| 1. Spec-valid VTODO (CRLF, folding, escaping) | PASS | `TestRenderTodo_Minimal`, plus the shared `logicalLines` CRLF/fold harness reused from `ical_test.go` |
| 2. All-day vs timed DUE | PASS | `TestRenderTodo_DueAllDayAndTimed` (table-driven, both directions asserted with negative cases) |
| 3. Completion trio; pending omits COMPLETED | PASS | `TestTodo_CompleteSetsTheWholeTrio`, `TestRenderTodo_Minimal` (asserts absence) |
| 4. No VEVENT/VTODO mixing | PASS | `TestRenderCollection_ComponentSelectsOneSlice` — a Feed with BOTH slices populated emits only the one matching `Component` |
| 5. ETag sensitive to completion, clock-independent | PASS | `TestTodoETag_StableAndSensitive` |
| 6. Apple fixture semantic equivalence | PASS (adapted — see below) | `TestRenderTodo_MatchesAppleSemantics` + 3 behaviour-pinning tests |
| 7. No new dependency | PASS | `just arch-lint` OK; imports unchanged |

**AC6 was implemented differently and this is deliberate.** The AC asked the
fixtures to "parse to the model and re-render." `calfeed` has **no parser** — it
is render-only, and parsing VTODO belongs to the go-webdav adapter (TKT-MF1CWZ).
Adding a parser here purely to satisfy a round-trip assertion would put a second
iCalendar implementation in the package whose vendor-free status is what keeps
the adapter boundary clean.

Instead the fixture tests assert what this layer can actually own: that our
rendering of the equivalent model **matches Apple's property-for-property**
(`TestRenderTodo_MatchesAppleSemantics`), plus three tests that pin observed
Apple behaviour as executable documentation:

- `TestApple_CompletedTodoShape` — the completion trio, and that `URL` survives
a round-trip (normalised to `URL;VALUE=URI:`)
- `TestApple_AddsDTSTARTMirroringDUE` — Apple invents a `DTSTART` equal to
`DUE`; a future diffing consumer must not read that as user intent
- `TestApple_ClientCreatedTodoIsBareUUID` — the fact that makes the alias
service mandatory (TKT-WAA092), and that a created to-do carries only `SUMMARY`
and `STATUS`

Rationale is in the test file's doc comment so a reviewer sees the reasoning
rather than assuming an oversight.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Notes:

- **DRY:** `RenderTodo` reuses `writeProp` / `writeLine` / `foldLine` /
`escapeText` / `formatDate` / `formatDateTimeUTC` verbatim. The VALARM block is
~7 lines duplicated from `RenderEvent`; left inline rather than extracted, since
a shared helper would need to take the fallback-description source and the gain
is thin at two call sites.
- **A design addition beyond the ticket:** `Todo.Complete(at)` sets the whole
`STATUS`/`COMPLETED`/`PERCENT-COMPLETE` trio in one call. The ticket described
these as three independent fields; making them independently settable invites
exactly the split-brain the research warned about (RFC 4791 §7.8.9 filters on
`COMPLETED`, UIs read `STATUS`), so a half-set state reads as done in one client
and pending in another.
- **Security:** `TestRenderTodo_NoLineBreakInjection` mirrors the existing
VEVENT guard — a CRLF in user content cannot forge a property line.
- **Build-hygiene addition:** `.gitattributes` pins `*.ics -text`. Without it
git normalises CRLF→LF on checkout, silently invalidating fixtures whose entire
value is being byte-exact. Verified CRLF survives into the index.

## Automated checks

- `go test ./internal/calfeed/` — PASS, **94.8% coverage** (floor 50%)
- `golangci-lint run ./internal/calfeed/` — **0 issues**
- `just arch-lint` — OK, no new component or vendor grant needed
- `go build ./...` — OK
- `internal/dataentry` + `internal/dataentryconfig` — PASS (no regression in
the existing `feeds:` consumers)

Commit: `d57d186d` on `feat/caldav-vtodo-renderer`.
