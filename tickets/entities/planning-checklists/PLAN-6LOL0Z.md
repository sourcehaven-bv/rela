---
id: PLAN-6LOL0Z
type: planning-checklist
title: 'Planning: Phase 1: calendar/feed export — internal/feed model, iCal+JSON renderers, rela.calendar Lua binding, HTTP feed endpoint'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Auth reconciled (RES-1X4NS9):** Phase 1 = loopback-trust only (no feed auth,
> no guard; RR-7C151B wont-fix). Networked access = future pratique scoped-token +
> read-only-CalDAV (pratique FR filed). iCal serializer built **event-granular** so
> CalDAV reuses it.
>
>
> **Lua API DECIDED (framework/provider interface):** a feed script returns
> `{ meta?, list(opts)→(events,cursor), get(uid)→event|nil }`; rela drives it for
> every transport, derives ETag/ctag, owns the sync cursor. Full spec:
> `provider-contract.md`. This **replaces the RR-78OHN5 stdout seam** — the script
> returns a table (like `ExecuteAction`), rela owns rendering.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**In scope (Phase 1):** (1) `internal/feed` pure model + **event-granular
CalDAV-ready** iCal serializer (VEVENT+VALARM, no vendor) + JSON; (2)
`rela.calendar` Lua **provider interface** (`{meta?, list, get}` — see banner +
`provider-contract.md`)
+ pure `event{}`/`alarm{}` constructors; (3) `ExecuteFeed` seam (runs the script,
takes its returned provider table; write-denying Mutator) + `feeds:` config on
`dataentryconfig.Config` (hot-reload + fail-fast script-exists); (4) `GET
/api/v1/_feeds/{name}.{ics,json}` on the **inner `/api/` mux** (inherits ACL
read gate + CSRF chain), CSRF-exempt via `nonBrowserExemptPrefixes`,
**loopback-trust only** + a **principal-resolution seam**.

**Auth/transport (RES-1X4NS9):** Phase 1 loopback-only; networked auth is a
future pratique scoped/revocable token as **HTTP Basic** to a **read-only
CalDAV** endpoint (pratique FR:
`../pratique/docs/fr-scoped-long-lived-tokens.md`).

**Out of scope:** read-only CalDAV + pratique-token auth (future, on Phase 1's
`RenderEvent`/`ETag`/`CollectionTag` + `list`/`get`); CalDAV two-way sync +
`VTODO` (Phase 2); Atom/RSS; `RRULE`; swiftbar/`launchd` glue.

**Acceptance Criteria:**

1. **Spec-valid VCALENDAR** — `RenderCollection` begins `BEGIN:VCALENDAR`, ends
`END:VCALENDAR\r\n`, CRLF, `VERSION:2.0`+`PRODID`, one `VEVENT` per event.
2. **All-day → `VALUE=DATE`**; timed → `DTSTART:…T…Z`.
3. **Stable UID + DTSTAMP** — `UID`=`TSK-001@rela`, byte-identical; DTSTAMP UTC via
injected clock (no `time.Now()` in serializer).
4. **Escape + fold (RFC 5545)** — `,;\`+newline escaped; >75-octet fold+unfold;
multibyte on octet boundary; 75/76 pinned.
5. **VALARM present iff an alarm**.
6. **JSON faithful**.
7. **Event-granular API (CalDAV-ready)** — `RenderCollection` = `VCALENDAR` envelope
wrapping `RenderEvent` per event; `ETag(e)` stable, changes iff content changes;
`CollectionTag(f)` changes iff any event does.
8. **Provider interface drives all transports** — a feed script `{list, get}`:
`list({})` → events (+ cursor) renders the ICS/JSON collection; `get(uid)`
returns one event or nil; the same uid yields the same event from both;
`list({since=c})` with an honored cursor returns only changed events, and a feed
that ignores `since` still renders correctly. (Pins transport-independence +
advisory delta.)
9. **Lua validates loudly** — missing `list`/`get` → feed load error; `event{}`
missing required → raise naming field; bad format → error; non-table event →
type error.
10. **Endpoint serves output** — `GET …/tasks.ics` → 200 `text/calendar…`;
`…/tasks.json` → 200 JSON; unknown feed → 404; unknown ext → 404/406.
11. **ACL-gated + CSRF-exempt** — `list`/`get` run under the read gate (inner mux);
bare non-browser request succeeds; browser-shaped hits same-origin.
12. **Config validates at load** — missing script → error; bad format → error;
hot-reload on edit.
13. **Backend isolation holds**.
14. **arch-lint + plimsoll + coverage pass**.

## Research

- [x] `/research` run · [x] libraries searched · [x] codebase patterns · [x]
reference impls · [x] rela concepts reviewed

**Research Docs:** RES-AHY3VS (model+renderers), RES-1X4NS9 (auth/transport).
**Contract note:** `provider-contract.md` (the `{meta,list,get}` spec + cursor +
drive table).

**Existing Solutions:** iCal libs rejected (small format, hand-roll); prior art
`internal/cli/export.go`, `internal/tracer`, `internal/lua/markdown.go` (the
`event`/`alarm` constructor idiom), `ExecuteAction` (script returns a table —
precedent for the provider return), `PLAN-3E5HR` (`rela.url`), `dataentryconfig`
`documents:`, `config.Loader`+`script.Engine`, `isCSRFExempt`, the postgres
watcher `max(seq)` watermark (precedent for the sync cursor). CalDAV = RFC
4791/6578.

## Approach

- [x] approach chosen · [x] builds on patterns · [x] alternatives · [x] deps

**1. `internal/feed`** — event-granular CalDAV-ready serializer:
```go
type Alarm struct { Trigger, Description string }
type Event struct { UID, Summary, Description, URL string; Start, End time.Time; AllDay bool; Alarms []Alarm }
type Feed  struct { Name, Domain string; Events []Event }
type ICal  struct { ProdID string; Now time.Time }   // Now injected
func (ICal) RenderEvent(Event) []byte                // CalDAV GET/multiget
func (ICal) RenderCollection(Feed) []byte            // ICS feed + calendar-query
func (ICal) ETag(Event) string
func (ICal) CollectionTag(Feed) string
func RenderJSON(Feed) ([]byte, error)                // collection-only
```

`RenderCollection` wraps `RenderEvent`. No `store`/`metamodel` import.

**2. `rela.calendar` provider interface** — DECIDED (see banner +
`provider-contract.md`). Script returns `{ meta?, list(opts)→(events,cursor),
get(uid)→event|nil }`. Both callbacks required; both transports use both.
`event{}`/`alarm{}` = pure tagged-table constructors, registered in
`registerReadBindings`. rela derives ETag/ctag, owns the opaque round-tripped
sync cursor (script-defined watermark, advisory `opts.since`, `>=`-inclusive
guidance, deletions inferred by rela's ETag-diff). No stdout seam.

**3. HTTP endpoint** — inner `/api/` mux (RR-4AWSTN); `ExecuteFeed` runs the
script, takes its returned provider table (RR-78OHN5 stdout seam dropped);
write-denying Mutator (RR-4C2AI4); format from URL ext; CSRF-exempt via
`nonBrowserExemptPrefixes`; principal seam (Phase 1 = loopback/default).

**4. Config** — `Feeds map[string]FeedConfig{Script, Format, Domain}`;
`validateFeeds` mirrors `validateDocuments`.

**Files:** `internal/feed/{feed,ical,json}.go` (+tests);
`internal/lua/calendar.go` (+test) + `runtime.go`; `internal/script/executor.go`
(`ExecuteFeed` returning the provider table);
`internal/dataentryconfig/{config,validate}.go`; `internal/dataentry/api_v1.go`
+ `feeds.go` + `middleware_security.go` + `router_walk_test.go`;
`.go-arch-lint.yml`; docs.

**Alternatives rejected:** library `render()` shape (couples model to transport,
needs stdout seam, CalDAV re-runs+discards); A3 YAML feed query; B2 renderer
plugin registry; B3 iCal lib. **Auth (RES-1X4NS9):** proxy-trust dropped;
per-feed token → future pratique path; read-only CalDAV = future transport on
this serializer + provider.

## Security Considerations

- [x] inputs · [x] allowlist validation · [x] sensitive ops · [x] no leak

- **Feed name+ext** — allowlist (map key, ext ∈ {ics,json}); else 404. No traversal.
- **`list`/`get` output** — writer runtime + **write-denying Mutator** under the
**ACL read gate** (inner mux); only readable entities surface; errors → 500.
- **`feeds:` config** — `filepath.IsLocal`+`.lua`+exists; format/domain allowlisted.
- **iCal text** — escaped; fold counts octets.
- **Sync cursor** — rela-opaque (never parsed/eval'd); a script can't inject via it.
- **Auth (Phase 1)** — loopback-trust; CSRF-exempt via existing `isCSRFExempt`; no
feed-specific auth code, **no new secret store**. Existing bind +
`shouldWarnNoACL` warnings cover unsafe binds (RR-7C151B wont-fix).

## Test Plan

- [x] scenarios · [x] edge cases · [x] negative · [x] integration

**Scenarios:** AC1–9 → `internal/feed`+`internal/lua` unit tests (pure, injected
clock; AC7 pins collection=Σ RenderEvent + ETag stability; AC8 pins `list`/`get`
consistency + a since-honoring feed vs a since-ignoring feed both correct).
AC10–12 → `internal/dataentry` handler tests + `dataentryconfig` validate +
hot-reload. AC13 → CI isolation. AC14 → lint/coverage. **Integration:** boot
with a `feeds:` config + real provider script, `GET` `.ics`/`.json`, assert
bodies + ACL scoping.

**Edge Cases:** no/multi-alarm; all-day vs timed; `End`; zero events; fold
cases; >1 event/entity → discriminated UID; `get(uid)` == that uid's event in
`list`; `list` returning nil cursor (no-delta feed); missing `due` → skipped by
both callbacks; concurrent fetches (one LState/goroutine); feed removed → 404.

**Negative:** unknown feed → 404; bad ext → 404/406; bad format → error;
`event{}` missing required → Lua error; missing `list`/`get` → load error;
missing script → load error; script runtime error → 500.

## Risk Assessment

- [x] technical · [x] security · [x] effort

- **iCalendar interop** — RFC-5545 test table; known-good reference; manual Apple
Calendar subscribe (loopback).
- **Cursor boundary bug** — `>=`-inclusive + ETag-diff absorbs overlap; test-pinned.
- **Provider contract is a new pattern** — pinned by `provider-contract.md` + AC8.
- **arch-lint/plimsoll** — split files; minimal deps.

**Effort:** `l`.

## Documentation Planning

- [x] user docs identified · [x] docs-checklist at `review`

- [x] Guide "Calendar & feed export" (config, the `{meta,list,get}` provider, loopback
subscribe; networked/token noted future).
- [x] `docs/data-entry.md`, [x] `CLAUDE.md` note, [x] ~~README~~ N/A, [x] docs-checklist
at review.

## Design Review

- [x] `/design-review` run · [x] critical/significant addressed (auth superseded by
RES-1X4NS9; RR-78OHN5 superseded by the provider-return shape)

**Findings:** RR-4C2AI4 (read gate + write-deny) · RR-4AWSTN (inner mux) ·
RR-FZRAC8 (route catalogue+ReadDeps) · RR-0E20T7 (iCal test pins) — folded in.
**RR-78OHN5 (stdout seam) — superseded**: the provider script returns a table,
not a blob, so there's no stdout capture. **RR-7C151B (proxy-trust+guard) —
wont-fix, superseded** by RES-1X4NS9.
