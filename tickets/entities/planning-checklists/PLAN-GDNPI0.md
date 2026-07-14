---
id: PLAN-GDNPI0
type: planning-checklist
title: 'Planning: Timed calendar-feed events from datetime sources (DTSTART with time, datetime start/end range)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** The calendar-feed export (Phase 1, TKT-RDM9M5, done) emits
**all-day events only** (`DTSTART;VALUE=DATE`) because no time-bearing property
type existed. `datetime` now exists (TKT-ZYOBSN). Make the feed **type-driven**:
a `datetime` feed source emits a **timed** event; a `datetime` start+end pair
emits a timed-range VEVENT. `date` sources stay all-day (unchanged).

**Scope — IN:**
- Allow a feed `date:` / `end_date:` source property to be `date` **or** `datetime` (relax the `validate_feeds.go` gate at :89 and :127).
- **Reject a start/end type mismatch** at config-validation: `date:` and `end_date:` must be the same kind (both all-day or both timed) — iCal forbids mixing `VALUE=DATE` DTSTART with a timed DTEND.
- `calfeed.Event` gains an **`AllDay bool`** discriminator (Start/End already carry the full `time.Time`).
- `calfeed` iCal serializer: timed `DTSTART:`/`DTEND:` (`YYYYMMDDTHHMMSSZ`, UTC, via existing `formatDateTimeUTC`) when `!AllDay`; `VALUE=DATE` when `AllDay`.
- `calfeed` JSON serializer: `AllDay` from the flag; timed values render as RFC3339 (not date-only) when timed.
- `feed_provider.mapEntity`: set `AllDay` from `entDef.Properties[s.Date].Type` (`date` → true, `datetime` → false). No `ParseDateValue` change (already parses time-of-day for datetime).
- Tests: calfeed timed iCal + JSON, validate_feeds accept-datetime + reject-mismatch, feed_provider datetime→timed mapping, handler end-to-end.
- Docs: feed docs + flip the metamodel "timed events are a planned follow-on" note.

**Scope — OUT (follow-on):**
- **Timezone in the feed** (`TZID`/`VTIMEZONE`) — emit UTC `Z` only, matching Phase 1. A per-feed display tz is a later enhancement.
- CalDAV, RRULE authoring, VTODO — later phases (unchanged).
- Lua feed provider — unchanged.

**Acceptance Criteria:**
1. A feed source whose `date:` property is `datetime` loads clean (no "must be date-typed" error). *Test:* `validate_feeds_test.go` valid-case with a datetime `date:`.
2. A feed source with `date:` and `end_date:` of **different** kinds (one `date`, one `datetime`) is a metamodel-load error. *Test:* `validate_feeds_test.go` mismatch error case (both directions).
3. A `datetime` source renders a **timed** `DTSTART:YYYYMMDDTHHMMSSZ` (no `VALUE=DATE`), and a `date` source still renders `DTSTART;VALUE=DATE:YYYYMMDD`. *Test:* `ical_test.go` timed sibling of `TestRenderEvent_AllDayValueDate` (assert timed line present, `VALUE=DATE` absent) + the existing all-day test still passes.
4. A `datetime` start+end renders timed `DTSTART` + timed `DTEND`. *Test:* `ical_test.go` timed-range case.
5. JSON output sets `AllDay:false` + a time-bearing `date`/`end_date` for a timed event; `AllDay:true` + date-only for all-day. *Test:* `json_test.go` timed round-trip.
6. `feed_provider.mapEntity` produces a timed `Event` (AllDay=false, Start carries time-of-day) for a datetime source and an all-day one for a date source. *Test:* `feed_provider_test.go` datetime-source case mirroring the existing all-day tests.
7. End-to-end: `GET /_feeds/{name}.ics` for a feed with a datetime source returns a valid VCALENDAR with a timed VEVENT. *Test:* `feed_handler_test.go` datetime feed case.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (Phase 1 already carries RES-AHY3VS/RES-1X4NS9; a focused
Explore sweep of `calfeed`/`feed_provider`/`validate_feeds` captured below is
sufficient for this type-driven extension).

**Existing Solutions:**
- **Phase 1 (TKT-RDM9M5)** built the whole feed subsystem and explicitly anticipated this: "Timed events... picked up type-driven once `datetime` lands." `formatDateTimeUTC` (ical.go:233) already exists (used for DTSTAMP) — the timed `DTSTART` reuses it. `Start`/`End` on `calfeed.Event` already hold the full `time.Time`; only the renderers truncate to a day.
- **Phase 1 iCal-correctness review pins** (RR-0E20T7 DTSTAMP/UID/CRLF, RR-2V0019 CRLF injection) — the fold/escape/CRLF path is type-agnostic and already tested; timed DTSTART flows through the same `writeLine`, so those invariants hold. ETag/CollectionTag auto-follow (they hash `RenderEvent` output).
- **The datetime type (TKT-ZYOBSN)** stores UTC RFC3339; `ParseDateValue` returns a full instant for a datetime prop, so the provider already has the time-of-day.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

> **POST-DESIGN-REVIEW (2026-07-14):** 6 findings (2 critical). Three plan
> decisions changed — reflected below:
> - Discriminator is **`Timed bool`** (zero-value = all-day), NOT `AllDay bool`
>   (RR-NZ2I90) — the safe default; no existing Event literal or ETag changes.
> - **`End` renders verbatim (exclusive)** for both types — the branch changes
>   only the FORMAT, never `End`'s meaning (RR-JC3XMY).
> - JSON keeps `date`/`endDate` **date-only**; adds separate `start`/`end`
>   RFC3339 fields for timed events (RR-CT338H) — backward-compatible.

**Technical Approach (5 touch points + tests + docs):**
1. `internal/calfeed/calfeed.go` — add **`Timed bool`** to `Event` (~:35-59). Zero-value = all-day (backward-compatible; no existing literal needs editing). Document: `Timed` false → `VALUE=DATE`; true → timed UTC instant. Update the `End` doc comment: "rendered verbatim as the RFC5545 DTEND value (exclusive) for both all-day and timed." `Start`/`End` unchanged (already `time.Time`).
2. `internal/calfeed/ical.go` `RenderEvent` (~:66-98) — branch DTSTART (:71) and DTEND (:72-74) on `e.Timed`: false → `DTSTART;VALUE=DATE:`+`formatDate` (byte-identical to today, so ETags stable); true → `DTSTART:`+`formatDateTimeUTC` (UTC `Z`). No new helper needed. The branch changes only format — `End` is rendered verbatim either way (no +1 adjustment anywhere).
3. `internal/calfeed/json.go` `RenderJSON` (~:40-66) — set `jsonEvent.AllDay` from `!e.Timed`; keep `Date`/`EndDate` **strictly date-only** (all-day only); add new `Start`/`End` RFC3339 fields populated **only when timed**. Existing date-only consumers unaffected.
4. `internal/dataentry/feed_provider.go` `mapEntity` (~:194-245) — set `ev.Timed = (dateDef.Type == metamodel.PropertyTypeDatetime)` (`dateDef` already in scope at :209). `End` set verbatim as today (no branch). (Same-kind enforced by validation, so no separate end branch needed.)
5. `internal/dataentryconfig/validate_feeds.go` `validateFeedSource` (~:75-150) — (a) add `isFeedDateType(t)` helper (accepts `date` OR `datetime`), use at **both** the `date:` gate (:89) and `end_date:` gate (:127); (b) add a **same-kind mismatch check**: if both `date:` and `end_date:` are set and their types differ, error naming **both properties and both types**, matching the `fmt.Sprintf("%s: ...", prefix, ...)` style (e.g. `"%s: date property %q is %q but end_date property %q is %q — a feed event must be all-day or timed, not both"`).

**Alternatives considered:**
- *Start-wins coercion for a start/end mismatch* — rejected (per decision): silently drops the time or the date; reject at validation is spec-correct and fail-fast, matching the existing gate style.
- *`AllDay bool` on the Event struct* — **rejected after design review (RR-NZ2I90)**: zero-value would be `false`=timed, silently breaking ~20 existing all-day Event literals into `T000000Z` and churning every existing feed's ETag. `Timed bool` makes the zero-value all-day (safe/backward-compatible). The JSON model keeps its `AllDay` field (set from `!e.Timed`) so the wire name is unchanged.
- *Overload JSON `date`/`endDate` with RFC3339 when timed* — **rejected (RR-CT338H)**: breaks consumers parsing the documented date-only shape. Separate `start`/`end` RFC3339 fields instead.
- *Adjust `End` (+1 day) or coerce per type* — **rejected (RR-JC3XMY)**: `End` renders verbatim/exclusive for both; the branch changes only format. Coercion would introduce a per-type off-by-one.
- *Emit `TZID`/local time* — out of scope; UTC `Z` matches Phase 1 and the datetime type's UTC storage.
- *Infer timed-ness from whether Start has a non-midnight time* — rejected: a legitimately-midnight datetime would be misclassified as all-day; the property **type** is the correct, unambiguous signal.

**Files to modify:** calfeed/calfeed.go, calfeed/ical.go, calfeed/json.go,
dataentry/feed_provider.go, dataentryconfig/validate_feeds.go; tests:
calfeed/ical_test.go, calfeed/json_test.go, dataentry/feed_provider_test.go,
dataentry/feed_handler_test.go, dataentryconfig/validate_feeds_test.go; docs:
docs-project source guides for data-entry (feeds) + the metamodel datetime note
(regenerate). Update the `FeedSource`/`Event` doc comments that say "all-day
(Phase 1)".

**Dependencies:** none new. Uses `PropertyTypeDatetime` (already exists) and
`formatDateTimeUTC` (already exists).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- *Feed source config* (`feeds:` in data-entry.yaml): validated at load — `date:`/`end_date:` must be `date` or `datetime` (allowlist), and same-kind (mismatch rejected). Fail-fast, like the existing gate.
- *Datetime property value* (already validated by the datetime type): a timed `DTSTART` is derived from a stored UTC instant; **CRLF/escape is unchanged** — the value flows through the same `writeLine`/`formatDateTimeUTC` path (a time.Time formatted to `YYYYMMDDTHHMMSSZ` cannot contain CRLF or injection chars), so the Phase 1 injection guards (RR-2V0019) are not weakened. No user string reaches the DTSTART line unescaped.
- No new I/O, auth, or crypto. The feed endpoint's ACL read gate + loopback trust are unchanged (type-agnostic).

**Security-Sensitive Operations:** none new. A formatted timestamp is not
attacker-controllable text.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (per AC):** see AC list — each names its file. Mirror the
existing all-day tests:
- `ical_test.go`: timed `DTSTART`/`DTEND` (assert timed line, assert **no** `VALUE=DATE`), keep the all-day negative-assertion test passing.
- `json_test.go`: timed round-trip (`AllDay:false`, RFC3339 date).
- `validate_feeds_test.go`: datetime accepted; start/end mismatch rejected (both directions); the existing "datetime rejected" cases updated.
- `feed_provider_test.go`: datetime source → `AllDay:false` Event with time-of-day; date source → `AllDay:true`.
- `feed_handler_test.go`: end-to-end ICS for a datetime feed is a valid VCALENDAR with a timed VEVENT.

**Edge Cases:**
- Datetime start, **no** end → single timed event (no DTEND), like the all-day single-day case.
- Datetime value at exactly midnight UTC → still **timed** (`DTSTART:...T000000Z`), because the property type says timed — NOT misclassified as all-day (the reason we key on type, not the time value).
- Mixed `date` start + `datetime` end (and reverse) → validation error (AC2).
- ETag/CollectionTag change when an event flips all-day↔timed (they hash RenderEvent output — verify sensitivity).
- Empty/unparseable datetime value → skipped, same as the existing `TestDeclarativeFeed_UnparseableDateSkipped` path (unchanged).

**Negative Tests:** start/end type mismatch → load error; unparseable value →
event skipped; a `date`-typed source still emits `VALUE=DATE` (no regression).

**Integration approach:** `feed_handler_test.go` exercises HTTP → provider →
mapEntity → RenderCollection for a datetime feed. Manual E2E: add a `datetime`
prop + a feed source to a scratch project, curl the `.ics`, confirm a timed
VEVENT; validate with an iCal linter if available.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- *Breaking the Phase 1 all-day invariant* → **Mitigation:** `AllDay` defaults to false on a zero Event, so the provider must set it explicitly for date sources; the existing all-day test (with its `VALUE=DATE` + no-bare-`DTSTART` assertions) guards the date path. Verify date sources still get `AllDay:true`.
- *iCal malformity from a mixed event* → **Mitigation:** the same-kind validation makes a mixed event unconstructable.
- *ETag churn flipping a feed* → **Mitigation:** intended (the event genuinely changed); tests assert ETag sensitivity.
- *Midnight-UTC misclassification* → **Mitigation:** key on property type, not the time value (documented).

**Effort:** **S–M** — 5 small, well-localized touch points (the plumbing already
carries the instant) + mirrored tests + a docs regen.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] data-entry feed docs (source guide) — `date:`/`end_date:` may be `date` or `datetime`; datetime → timed event; same-kind rule; UTC-only.
- [x] metamodel datetime note — flip "Calendar-feed sources currently accept only `date`... timed events are a planned follow-on" to reflect that datetime sources now emit timed events (edit `docs-project/` source + regenerate).
- [ ] docs/cli-reference.md — N/A.
- [ ] CLAUDE.md — N/A.
- [ ] README.md — N/A.

**Note:** docs are auto-generated from `docs-project/entities/` — edit the
source guides and run `just docs`, never the generated `docs/*.md` directly
(learned in TKT-ZYOBSN).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (2026-07-14):** 6 findings — 2 critical, 3 significant,
1 nit. All addressed in the plan; verified against the code.

| RR | Sev | Finding | Resolution |
|----|-----|---------|-----------|
| RR-NZ2I90 | critical | `AllDay bool` default is backwards (breaks ~20 literals + ETags) | Use **`Timed bool`** (zero-value = all-day) |
| RR-JC3XMY | critical | DTEND meaning must not diverge all-day vs timed | `End` renders **verbatim/exclusive**; branch changes only format |
| RR-1KKN1N | significant | Relax BOTH gates; mismatch error must name props+types | `isFeedDateType` helper at :89 + :127; same-kind check names both |
| RR-23YS91 | significant | Tests added-from-zero, not flipped; keep all-day test separate | Add datetime props to feed test metamodel + new `TestRenderEvent_Timed` |
| RR-CT338H | significant | JSON `date` overload breaks consumers | Keep `date`/`endDate` date-only; add `start`/`end` RFC3339 fields |
| RR-W2JQKA | nit | non-UTC input UTC-converted; `day()` helper clarity | `Timed bool` fixes ambiguity; add `dayTime()` helper; doc the UTC note |
