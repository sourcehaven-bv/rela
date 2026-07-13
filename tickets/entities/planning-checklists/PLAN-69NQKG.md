---
id: PLAN-69NQKG
type: planning-checklist
title: 'Planning: Add a datetime metamodel property type (time-bearing, with date+time form widget)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** The metamodel has a `date` (date-only) property type but no
time-bearing `datetime` type, and no date+time form widget. Authoring a
timestamp means hand-editing RFC3339 into a date field. Add a first-class
`datetime` property type + a data-entry widget that captures date **and** time,
with clear, adjustable timezone communication.

> **POST-DESIGN-REVIEW (2026-07-13):** Design review (9 findings, see Design
> Review section) changed three decisions from the original plan. This
> Understanding/Scope block reflects the **final** post-review scope:
> - Bare dates are **accepted as midnight-UTC**, NOT rejected (RR-MYC2B6).
> - Validation accepts **both `string` and `time.Time`** (yaml auto-decodes
>   unquoted datetimes to `time.Time`) (RR-NY7PRB).
> - The inline tz control is a **read-only indicator** in v1; the picker lives
>   in **Settings** (RR-3A0RUL). `@date-fns/tz` retained per user (RR-35VJ8G).

**Scope — IN:**
- New `PropertyTypeDatetime = "datetime"` builtin, registered as a builtin type.
- Validation: a `datetime` value is any RFC3339-parseable instant. The arm accepts a Go `string` (parse via `ParseDateValue`) **or** a `time.Time` (yaml already decoded it) — a bare date is accepted and means midnight-UTC.
- Typed filter (`<` `<=` `>` `>=`) and sort support, reusing the existing date `time.Time` comparison core. datetime shares `typeRankDate` so date+datetime interleave chronologically. `=` is strict-instant.
- OpenAPI schema arm (`format: date-time`).
- Type→widget resolution (`ResolveWidgetFromType` → `"datetime"`, backend widget const, frontend registry).
- A `DatetimeWidget.vue`: native `<input type="datetime-local">` for edit, formatted display for view. **Non-destructive**: emits a new value only when the user edits *this* field; untouched values pass through verbatim (no incidental `+02:00`→`Z` churn).
- **Read-only inline tz indicator** beneath the input ("Times shown in <effective IANA tz>"). The actual **display-timezone picker lives in Settings** (next to theme), listing `Intl.supportedValuesOf('timeZone')` + a "Browser default" top entry, keyboard + screen-reader accessible. The pref is global, persisted in **`localStorage` via the Pinia `uiStore`** (never sent to the server).
- Storage semantics: widget captures wall-clock in the effective zone, converts to **UTC RFC3339** (`...Z`) on user edit, converts UTC→effective-zone for edit/display.
- Uses **`@date-fns/tz`** (`TZDate`) for correct wall-clock↔UTC conversion in a *named* zone. (Impl task: `npm audit` clean + note transitive footprint.)
- Go + frontend unit tests mirroring the date-type tests, plus tz + DST tests.
- Docs: `docs/metamodel.md` (new type), `docs/data-entry.md` (widget + tz display).

**Scope — OUT (follow-on, each a documented decision — not silence):**
- Timed calendar-feed events. `validate_feeds.go:89,127` stays date-only (datetime NOT feed-eligible yet); calfeed stays all-day. Tracked by TKT-RDM9M5 / FEAT-OT4361 — this ticket is the metamodel primitive only (RR-FF45CP).
- A *server-side* / per-project timezone setting. The override is a pure client display preference; no backend tz config.
- **Clickable** inline tz shortcut (v1 inline is read-only; picker is in Settings) (RR-3A0RUL).
- Seconds-precision UX polish across mobile browsers.
- `datetime` as an entity `display_property` — treat like `date` (rejected in `loader.go`).
- date→datetime **migration**: not needed — bare-date acceptance makes the widening lossless (RR-MYC2B6).

**Acceptance Criteria:**
1. A metamodel property declared `type: datetime` is accepted as a builtin. *Test:* `IsBuiltinType("datetime") == true`; metamodel with a datetime prop loads clean.
2. Validation accepts RFC3339 timestamps AND `time.Time` values AND bare dates (as midnight-UTC); rejects true junk. *Test:* `validation_test.go` table: `"2026-07-13T14:30:00Z"` (string) valid; a `time.Time` value valid; `"2026-07-13"` valid (= midnight-UTC); `"not-a-date"` invalid; `""`/absent OK unless required.
3. Filtering a datetime prop with `>`/`<` compares as instants. *Test:* `match_test.go`: `2026-07-13T14:30:00Z` matches `>2026-07-13T09:00:00Z`, not `>2026-07-13T20:00:00Z`. Plus: `=2026-07-13` does NOT match `2026-07-13T12:30:00Z` (strict-instant, documented).
4. Sorting by a datetime prop orders chronologically incl. time; a MIXED date+datetime column also sorts chronologically (shared `typeRankDate`). *Test:* `sort_test.go` same-day-different-time + mixed-type cases.
5. OpenAPI schema for a datetime prop is `{type: string, format: date-time}`. *Test:* openapi assertion.
6. Type→widget resolves `datetime` → `datetime` widget end to end, incl. the relation-property modal render path. *Test:* Go `resolveWidget` table + frontend `registry.test.ts` + relation-modal render check.
7. `DatetimeWidget` edit mode renders `datetime-local`, round-trips a UTC value to the effective zone and back, emits `...Z`, and is non-destructive for untouched values. *Test:* `widgets.test.ts`: `2026-07-13T12:30:00Z` @ zone `UTC` → input `2026-07-13T12:30`; change to `13:30` emits `2026-07-13T13:30:00Z`; mounting without editing emits nothing.
8. Display mode renders a deterministic datetime in the effective zone. *Test:* `widgets.test.ts` display assertion using `Intl.DateTimeFormat({dateStyle:'medium', timeStyle:'short', timeZone})` + zone suffix.
9. **TZ display setting:** the Settings picker changes input interpretation + display and persists across reloads; stored values stay UTC/unchanged; the inline indicator reflects the effective zone (read-only). *Test:* `widgets.test.ts` (zone `America/New_York`: UTC `2026-07-13T12:30:00Z` → input `08:30`; edit emits correct UTC) + `ui.test.ts` (uiStore persist/read + `effectiveTimezone` fallback + stale-value fallback).
10. **DST/offset correctness.** *Test:* conversion helper tests for DST spring-forward, fall-back, `Asia/Kolkata` (+05:30), `Pacific/Auckland` (date line), and `Z`-in/offset-out round-trip.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (two Explore sweeps + a web-research sweep + a
design-review sweep captured below; medium change, not large enough for a RES
entity).

**Existing Solutions:**
- **Prior art in-repo:** `date` type + `DateWidget.vue` are the direct template — datetime mirrors every date type-switch arm; the widget mirrors DateWidget (single component, edit+display). `rrule` (FEAT-DUW9) is the precedent for a richer widget. `uiStore` (`frontend/src/stores/ui.ts`) is the persistence template — `theme` = a `ref` from `localStorage.getItem` persisted via `watch`; the tz pref follows the same shape (`datetimeTimezone` ref + init + `watch` → `localStorage`, `setDatetimeTimezone` action, `effectiveTimezone` getter).
- **Prebuilt Vue pickers:** `@vuepic/vue-datepicker` (~236KB + 4 deps) is the only maintained tz-aware picker, but its `timezone` prop does conversion only (no tz-name UI). **Rejected** for the full picker; we take only its `@date-fns/tz` `TZDate` primitive for named-zone↔UTC, and keep the native input + our own indicator.
- **No global tz/locale setting exists** in rela (verified: no backend config, no `.rela/user-defaults.yaml` field, no Pinia field; backend `$today` is UTC). The widget owns the tz story; the pref is client-only.
- **YAML decode behavior (verified by running yaml.v3):** an unquoted RFC3339 or bare-date scalar in frontmatter decodes to Go `time.Time`, not `string`; `markdown/parser.go:85` returns it un-normalized. This drives the "accept `time.Time`" validation decision and kills the "reject bare date" rule (both midnight-`time.Time`).
- **Native `datetime-local` gaps** (accepted): no tz in value/UI (indicator covers it), seconds opt-in & mobile-inconsistent (fine), cross-browser styling variance (minor).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Backend (Go)* — add `PropertyTypeDatetime` and slot a `datetime` arm into every
type-switch the terrain map found:
1. `internal/metamodel/types.go` — `PropertyTypeDatetime = "datetime"` const (~:273-281); add to `IsBuiltinType` (~:340); `DefaultDatetimeFormat = time.RFC3339` sibling (~:334).
2. `internal/metamodel/validation.go` — `case PropertyTypeDatetime:` arm (~:235) as a **type-switch on the value**: `case string:` → `ParseDateValue`; `case time.Time:` → accept (already an instant); empty/absent → OK unless required; else invalid-type error. **No bare-date rejection.**
3. `internal/metamodel/schema_output.go` — `ResolveWidgetFromType` → `"datetime"` (~:121).
4. `internal/openapi/schemas.go` — `{type: string, format: date-time}` (~:87). (Cosmetic; the yaml parser, not the schema, sets the Go type.)
5. `internal/filter/match.go` — dispatch to `matchDate` (~:73) + relational operators in `validateOperatorForType` (~:169). `matchDate` unchanged. `=` is strict-instant (documented).
6. `internal/filter/sort.go` — arms at `SortMulti` (~:40), `compareByPropDef` (~:330), and `typeRank` (~:358) returning **`typeRankDate`** (shared, so date+datetime interleave). Verify the cross-type compare path in `comparePropValues` runs `toTime` on both when one side is date and the other datetime.
7. `internal/affordances/env.go` — `scalarPredicateType` date arm (~:111) → `StringType`.
8. `internal/metamodel/loader.go` — add to `display_property` rejection list (~:406).
9. `internal/dataentryconfig/config.go` — `WidgetDatetime = "datetime"` (~:28); re-export from `internal/dataentry/config.go` (~:24).
10. `internal/testutil/fixtures.go` — `PropertyTypeDatetime` arm + `RandomDatetime()` (~:187) — **required** (unknown types fall through to `RandomString` and would mask bugs).
11. **Leave `internal/dataentryconfig/validate_feeds.go` untouched** (datetime not feed-eligible this ticket — documented decision).

*Frontend (Vue/TS):*
12. `frontend/src/types/schema.ts:22` — add `'datetime'` to the PropertyType union.
13. `frontend/src/widgets/types.ts` — add `'datetime'` to `WidgetHintKind` (~:76-84).
14. `frontend/src/widgets/registry.ts` — `defaultWidgetFor` (`datetime → 'datetime'`, ~:21), `hintKindToWidgetName` (~:37), register in `buildDefaultRegistry` (~:106) `supportedPropertyTypes: ['datetime']`; add import.
15. `frontend/src/utils/format.ts` — tz-aware helpers on `@date-fns/tz` `TZDate`:
- `browserTimeZone()` → `Intl.DateTimeFormat().resolvedOptions().timeZone`.
- `utcISOToLocalInput(iso, tz)` → `datetime-local` string (`YYYY-MM-DDTHH:mm`) for the wall-clock in `tz`.
- `localInputToUtcISO(local, tz)` → UTC RFC3339 (`...Z`) interpreting `local` as wall-clock in `tz`.
- `formatDatetime(iso, tz)` → deterministic display via `Intl.DateTimeFormat({dateStyle:'medium', timeStyle:'short', timeZone: tz})` + zone suffix; add a `type === 'datetime'` arm in `formatValue` (~:48).
16. `frontend/src/stores/ui.ts` — add `datetimeTimezone` ref (default `''`), init from `localStorage`, `watch` → `localStorage['datetimeTimezone']`, `setDatetimeTimezone(tz)` (validates against `Intl.supportedValuesOf('timeZone')`, ignores junk), `effectiveTimezone` getter (`datetimeTimezone || browserTimeZone()`). Export all.
17. `frontend/src/views/SettingsView.vue` — add the **display-timezone picker** (next to theme): a `<select>` of `Intl.supportedValuesOf('timeZone')` with a "Browser default" top entry, labeled "Display timezone (applies to all times)", wired to `uiStore.setDatetimeTimezone`, keyboard + a11y-labeled.
18. **New** `frontend/src/widgets/DatetimeWidget.vue` — mirrors `DateWidget.vue`: display → `formatDatetime(value, effectiveTimezone)`; edit → `<input type="datetime-local">` bound via `utcISOToLocalInput`/`localInputToUtcISO` against `effectiveTimezone`. **Non-destructive**: dirty-track; emit `...Z` only on real user input to *this* field; never on mount; untouched → original string passes through. Beneath it a **read-only** `<span>` "Times shown in {effectiveTimezone}" (not clickable in v1). Reads `effectiveTimezone` from `uiStore`.

**Storage semantics (the crux):** stored value is UTC RFC3339 (`...Z`) once
authored by the widget. On the backend a bare date decodes to midnight-UTC and
is stored verbatim (no server-side local-zone guessing — none exists). The
widget round-trips effective-zone-wall-clock ↔ UTC at the edge and is
non-destructive for values it didn't author, so switching the display zone
re-labels existing values without rewriting bytes. **Known cosmetic quirk
(documented):** a *pre-existing* bare/midnight-UTC value viewed in a western
zone displays on the previous evening (e.g. `2026-07-13T00:00:00Z` shows
`2026-07-12 20:00` in `America/New_York`) — inherent to interpreting a real
instant; the widget never *creates* such values.

**Alternatives considered:**
- *Reject bare dates* — rejected: unimplementable post-yaml-parse (midnight ≡ bare date as `time.Time`); would also break hand-edited/legacy files. Accept as midnight-UTC instead.
- *Clickable inline tz control* — rejected for v1: a global setting shown per-field is a mode error; picker in Settings + read-only inline indicator.
- *Reuse `date` type with a custom `Format`* — rejected: no distinct type for filter/sort/feed to key on.
- *Full `@vuepic/vue-datepicker`* — rejected: 236KB + 4 deps; take only its `@date-fns/tz` primitive.
- *`Intl`-only (no lib)* — considered (RR-35VJ8G); user elected to add `@date-fns/tz` upfront. `Intl`-only stays a documented fallback.
- *Store local wall-clock / store with offset* — rejected: UTC instant is unambiguous.
- *Separate `ParseDatetimeValue`* — rejected: `ParseDateValue` already parses RFC3339 + preserves time.

**Files to modify:** (backend) types.go, validation.go, schema_output.go,
openapi/schemas.go, filter/match.go, filter/sort.go, affordances/env.go,
metamodel/loader.go, dataentryconfig/config.go, dataentry/config.go,
testutil/fixtures.go; (frontend) types/schema.ts, widgets/types.ts,
widgets/registry.ts, utils/format.ts, stores/ui.ts, views/SettingsView.vue, **+
new** widgets/DatetimeWidget.vue; (deps) frontend package.json
(+`@date-fns/tz`); (docs) docs/metamodel.md, docs/data-entry.md. Plus mirrored
test files (+ ui.test.ts). **Left untouched (documented):** validate_feeds.go,
calfeed, internal/migration.

**Dependencies:** one new frontend dep — **`@date-fns/tz`** (relies on `Intl`
for the tz db; no bundled tzdata). Impl task: `npm audit` clean + note
footprint. No Go deps.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- *Datetime property value* (widget/API/hand-edited markdown): validated **allowlist-style** — must be a `time.Time` or an RFC3339-parseable string via `ParseDateValue`. Invalid → validation error via the normal metamodel path (400/422), no silent acceptance.
- *Filter operand* (query string): parsed the same way; malformed → filter error, not panic.
- *TZ pref* (Settings picker → localStorage): validated against `Intl.supportedValuesOf('timeZone')`; unrecognized/tampered value falls back to the browser zone (no throw). Client-only; never reaches the server.
- No file access, auth, or crypto. Error messages reference expected format only.

**Security-Sensitive Operations:** none. Value-type addition + a client display
preference; no new I/O or server trust boundary.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (per AC):** see AC list above — each names its test file. Key
additions from review: AC2 covers `string` + `time.Time` + bare-date + empty;
AC4 covers mixed date/datetime chronological sort; AC7 covers non-destructive
mount; AC10 covers the DST/offset matrix.

**Edge Cases:**
- Empty/missing → unset unless required (early-return in the arm; widget omits key).
- Bare date `2026-07-13` → **valid**, = midnight-UTC.
- `time.Time` inbound (unquoted yaml) → valid.
- Explicit offset `2026-07-13T14:30:00+02:00` → accepted (RFC3339), compared as instant; widget re-emits `...Z` **only if the user edits it**.
- DST spring-forward (nonexistent wall time) / fall-back (ambiguous) → resolution pinned by `TZDate`, asserted in tests.
- `Asia/Kolkata` (+05:30) and `Pacific/Auckland` (date line) → offset + date-shift correctness.
- Pre-existing midnight-UTC value in a western zone → displays prior evening (documented, tested as expected behavior).
- Stale/invalid localStorage tz → fall back to browser zone.

**Negative Tests:** non-parseable value → validation error; bad filter operand →
filter error not panic; datetime as `display_property` → loader rejects;
tampered tz pref → safe fallback.

**Integration approach:** existing metamodel-load + filter + sort suites
exercise the arms together (add datetime rows). Frontend: FieldRenderer dispatch
test confirms a `datetime` field renders `DatetimeWidget` end to end,
**including the relation-property modal path**. Manual E2E: create an entity
type with a datetime prop (entity + relation), edit via data-entry, confirm
round-trip; change the Settings tz and confirm existing values re-label (not
rewrite) and new edits store correct UTC; edit an unrelated field on an entity
with a `+02:00` datetime and confirm **no diff** on the datetime.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- *Missed a type-switch arm* → terrain map + design review enumerate all sites (incl. `validate_feeds.go`, fixtures); `IsBuiltinType` + a load test catches "unknown type".
- *TZ/DST conversion bugs* → `@date-fns/tz` `TZDate` + explicit DST/offset/date-line test matrix (AC10).
- *Git churn from incidental rewrites* → non-destructive widget (dirty-track); confirm per-property PATCH only sends changed fields.
- *Bare-date display surprise* → documented; widget never creates such values.
- *New dependency* → small, `Intl`-backed, `npm audit` gate.
- *Scope creep into calfeed* → explicitly OUT; `validate_feeds.go` untouched.

**Effort:** **M** — many small mechanical type-switch arms + one new widget
(tz-aware, non-destructive) + uiStore field + Settings picker + tests (incl.
DST) + docs + one small dep.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md — new `datetime` builtin (alongside `date`); UTC RFC3339 storage; bare-date = midnight-UTC; `=` is strict-instant.
- [x] docs/data-entry.md — the datetime widget, the Settings display-timezone picker (client-only, display-only), the bare-date display quirk, and that datetime feed sources are a follow-on.
- [x] ~~docs/cli-reference.md~~ (N/A: no command changes)
- [x] ~~CLAUDE.md~~ (N/A: no new cross-cutting convention)
- [x] ~~README.md~~ (N/A: not a project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (2026-07-13):** 9 findings — 2 critical, 5 significant,
2 minor. Two criticals were verified by running the yaml.v3 parser directly. All
addressed (8) or wont-fix with reason (1); no open critical/significant.

| RR | Sev | Finding | Resolution |
|----|-----|---------|-----------|
| RR-NY7PRB | critical | yaml decodes unquoted datetimes to `time.Time`, not `string` | Validation arm accepts `string` **and** `time.Time` |
| RR-MYC2B6 | critical | Bare-date rejection unimplementable post-parse | **Dropped**; bare date = midnight-UTC; no migration |
| RR-N1Z9BF | significant | View+save rewrites `+02:00`→`Z` (git churn) | Non-destructive widget; dirty-track; pass-through untouched |
| RR-5R3QFJ | significant | Mixed date/datetime sort lexical; `=` undefined | Share `typeRankDate`; `=` documented strict-instant |
| RR-FF45CP | significant | Feed gate rejects datetime | Leave gate date-only; feeds are the follow-on (documented) |
| RR-3A0RUL | significant | Global tz control shown per-field (mode error) | Picker in Settings; inline = read-only indicator (v1) |
| RR-35VJ8G | significant | Prefer Intl-only over `@date-fns/tz` | **wont-fix**: user elected the lib; `npm audit` gate; Intl fallback documented |
| RR-GBL60P | minor | Empty/required + relation-modal path | Early-return on empty; verify relation-modal render |
| RR-QIIAZB | minor | Fixtures fallthrough; display format; DST tests | `RandomDatetime` required; deterministic `Intl` format; DST test matrix |
