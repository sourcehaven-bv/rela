---
id: RES-AHY3VS
type: research
title: 'Format-agnostic calendar/feed export: Lua-built feed model, pluggable renderers (iCal + JSON), HTTP-served'
summary: 'Recommend a format-agnostic internal/feed package: Lua builds an abstract event model (functional tagged tables, matching markdown.go), rela renders to hand-rolled iCalendar (VEVENT+VALARM) + JSON, served at /api/v1/_feeds/{name}.{ics,json} behind a per-feed capability token driving the ACL read gate; events deep-link via the existing rela.url. VTODO/CalDAV deferred to Phase 2.'
status: done
---

## Problem

The rela graph holds entities with time-bearing properties (a PIM `task` with a
`due` date; an ISMS `measure` with a review date; a project `milestone`). Today
nothing surfaces those on a timeline or pushes them into the tools a user
already lives in. The concrete driving need: a daily nudge for open PIM tasks
that pulls the user back into the data-entry app.

The chosen shape is **not** a bespoke notification system but a **read-only feed
export**: rela emits its time-bearing data in standard feed formats that
existing clients (Apple/Google Calendar, feed readers, a swiftbar plugin) know
how to poll and alert on. rela stays the single source of truth; the calendar is
a read-only *view* whose events carry deep links back into the app.

The design must be **format-agnostic**: a script author describes an abstract
*feed of events*, and rela renders that model to any of several output formats.
Phase 1 ships **iCalendar (`.ics`, VEVENT + VALARM)** and **JSON**; the same
model is meant to later back Atom/RSS and a Phase-2 CalDAV server. Which
entities become events is **script-controlled** (curation lives in user Lua, not
in core), so the same machinery serves any rela project, not just PIM.

**Explicitly out of scope (Phase 2, separate research):** CalDAV two-way sync
and `VTODO`. Rationale recorded in the conversation: a read-only ICS
subscription cannot feed checked-off state back, so `VTODO` (whose entire point
is completable state) is actively misleading in a one-way feed — checking it off
in Apple Reminders silently un-checks on the next poll. `VEVENT` is honest about
what it is ("this is due on this day") and its `VALARM` alerting is bulletproof
across clients. Two-way sync is what earns `VTODO`, and that requires a CalDAV
server — a genuine stateful, authenticated, protocol-conformant subsystem worth
its own research. **The Phase-1 iCalendar/VEVENT serializer is a strict
prerequisite that Phase 2 reuses**, so this is sequencing, not throwaway work.

## Context

Grounding from a codebase survey (four parallel Explore passes over the Lua
runtime, HTTP/routing/auth, scheduler/config, and arch-lint/build-tags).

### Prior art already in-tree

- **Format-flag export precedent** — `internal/cli/export.go` already models
"one abstract shape → pluggable serializers" (`-f json|csv|yaml` over a common
`ExportEntity`/`FullExport` struct). This is exactly the model→renderers
pattern, endorsed and shipping.
- **`internal/tracer`** is the cleanest architectural precedent for the new
package: a read-only domain service that reads via the `store` interface and
produces result trees, importing only `store`. A feed model+query package sits
at the same layer.
- **`internal/lua/markdown.go`** is the template for the Lua API: constructors
return tagged plain tables (`rela.md.heading(2,"T")` → `{type="heading",...}`),
and a terminal `rela.md.render(ast)` walks them in Go to a string. The whole Lua
surface is **stateless/functional** — there is *no*
userdata/metatable/builder-object pattern anywhere in the codebase.
- **Deep-link URL scheme already solved** — `PLAN-3E5HR` / `TKT-4MFUK` added
`internal/frontendroutes` (route catalogue) and the `rela.url(path, params?)`
Lua helper (Phoenix-style path verification). The entity detail route is
`/entity/:type/:id`; the frontend's own `entityDetailHref` returns
`/entity/${type}/${id}`. Events embed this via `rela.url` — no new URL work.
- **`rrule-go`** (`github.com/teambition/rrule-go`) is already a vendored dep
(used by `metamodel` and `lua`) — relevant to `RRULE` for Phase-2 recurring
events; not needed for Phase 1.
- Related open tickets to coordinate with: **TKT-Y0M1** (date arithmetic +
RRULE helpers for Lua — a feed needs date classification), **FEAT-QAOV6**
(scheduled Lua task runner), **TKT-XKRH** (Markdown AST API — same
functional-table idiom).

### Constraints (CLAUDE.md + linters)

- **Consumer-side interfaces** at the call site; capability bundles
(`ReadDeps`/`WriteDeps`) split read vs write. A feed export is a **read**
concern → `ReadDeps` only, registered in `registerReadBindings`, so it is
available on both reader and writer runtimes.
- **Don't leak storage/parsing types** across boundaries — the feed model is a
domain DTO built from `entity.Entity`, not `*markdown.Document`/`*graph.Graph`.
- **Backend-agnostic is mandatory.** CI (`ci.yml` "Assert dependency isolation")
greps `go list -deps` to prove the default build never links pgx and the
postgres build never links bleve. The feed package must read only through the
`store.Store` interface and must never import `bleveindex`/`pgstore`/`memstore`.
- **arch-lint** (`.go-arch-lint.yml`, go-arch-lint v3): a new `internal/feed`
component needs its own `deps` block and must be added to the `mayDependOn`
whitelist of each consumer (`dataentry`, `lua`). A new vendor (e.g. an iCal
library) needs a `vendors:` entry + `canUse` grant.
- **plimsoll** god-object caps (40 methods / 20 exported / 20 fields) — keep the
model type and each renderer small; split rather than accrete.
- **Coverage** default package floor 50% — pure renderers unit-test easily above
it; no override needed.

### HTTP + auth surface (the crux)

- `cmd/rela-server` uses **stdlib `net/http` + `ServeMux`** (no framework); the
data-entry `App.NewRouter()` composes middleware. New routes register on the
inner `/api/` mux.
- **There is no login/token/session auth.** The model is: loopback bind +
same-origin/Host allowlist middleware + a `principal.Principal` stamped on the
context + an **ACL read gate** (`readGateFromContext` / `visibleReader`). **Any
read endpoint under `/api/` must go through the read gate or it leaks entities
the principal cannot see.**
- The same-origin middleware is the problem for feeds: **a calendar app
subscribing to a URL is not a browser** — no `Origin`/`Referer`, can't satisfy
same-origin. The **sync API is already CSRF-exempt** as a documented non-browser
path (`isCSRFExempt`); a feed endpoint needs the same exemption **plus** its own
authentication, because exempting it from same-origin removes the only thing
standing between a cross-origin fetch and the data. This is open question #4 and
the one genuine design decision left.
- **SSE feed pattern** (`handleSSE`/`startStoreEventBridge`) shows the
streaming-endpoint idiom (register outside the reload lock, per-connection ACL
gate, `WriteTimeout: 0` already set) — a feed is simpler (a plain GET), but the
ACL-gating precedent transfers directly.

### Config registry precedent

- `data-entry.yaml` (`dataentryconfig.Config`) already carries named
script-registries: `documents: map[string]DocumentConfig{Script,...}`,
`commands:`, `actions:`. Adding `feeds: map[string]FeedConfig` there is the
natural home — the server owns `data-entry.yaml`, gets **hot-reload for free**
(the watcher re-unmarshals `Config`), and the `documents:` entry (a named key →
a Lua `script:` that renders output) is a near-exact structural match.
- Scripts live under `<root>/scripts/`, referenced by relative path, loaded via
the hardened `config.Loader` + `script.Engine` (traversal-safe `os.OpenRoot`,
`.lua` required) — never `os.ReadFile`. Config-load does a fail-fast
script-exists check (`CheckDocumentScriptExists`-style).

## Options

Three axes have real alternatives: (A) the Lua API shape, (B) where the model +
renderers live and how renderers plug in, (C) the feed-endpoint auth model. (The
deep-link scheme, config home, and package layer are effectively decided by
existing precedent and noted as recommendations, not open options.)

### Axis A — Lua API shape

**A1. Functional tables + terminal render (matches `markdown.go`).
[recommended]** Event constructors return tagged plain tables; a terminal render
dispatches on a `format` option:
```lua
local events = {}
for _, t in ipairs(rela.list_entities("task", "status!=done")) do
  local due = t:prop("due")
  if due then
    events[#events+1] = rela.calendar.event{
      uid     = t.id,                       -- stable UID (see model notes)
      summary = t:prop("title"),
      date    = due,                        -- all-day → VALUE=DATE
      url     = rela.url("/entity/task/" .. t.id),
      alarm   = { trigger = "-PT9H" },      -- 09:00 the day before, etc.
    }
  end
end
return rela.calendar.render(events, { format = "ics" })   -- or "json"
```

- **Pros:** Idiomatic — zero new gopher-lua machinery (no userdata/metatables,
none exist today); trivially testable; `render(events, {format})` keeps the Lua
boundary format-agnostic (add a renderer without touching the Lua API); mirrors
`rela.md`. Scripts compose events with plain table ops.
- **Cons:** No compile-time schema on the table (bad keys are runtime
`RaiseError`); slightly more verbose than a fluent builder.
- **Effort:** S for the binding.

**A2. Stateful builder object (`f = rela.calendar.new(); f:add{...};
f:render()`).**
- **Pros:** Fluent; familiar OO ergonomics.
- **Cons:** **No precedent** — would introduce gopher-lua userdata + metatables
for the first time, against a strongly established functional idiom; more Go
code; harder to test. Rejected on consistency grounds.
- **Effort:** M.

**A3. Zero Lua — a declarative `feeds:` YAML query (no script).** Config names
an entity type + a due property + a filter; core builds events.
- **Pros:** No Lua for the simple case.
- **Cons:** Curation logic (overdue vs due-today classification, custom summary
text, which entities qualify) is exactly the open-ended part the user wants in a
script they own; a YAML DSL would grow to re-implement Lua. Rejected as the
primary path, but **could layer on later** as sugar over A1.
- **Effort:** M and open-ended.

### Axis B — Model location + renderer plug-in

**B1. New leaf `internal/feed` package; renderers as an internal closed set.
[recommended]** A domain package at the `internal/` top level (sibling of
`tracer`), importing only `entity`/`store`/`metamodel`/`tracer`/`filter`.
Defines the model (`feed.Feed`, `feed.Event`, `feed.Alarm`) and a small internal
renderer set (`renderICS`, `renderJSON`) selected by a `Format` value.
```go
package feed
type Event struct {
    UID, Summary, Description, URL string
    Start time.Time; AllDay bool
    Alarms []Alarm
}
type Feed struct { Name string; Events []Event }
func Render(f Feed, format Format) ([]byte, string /*contentType*/, error)
```

- **Pros:** Clean layer (mirrors `tracer`); backend-agnostic (store interface
only) so it links under all three build tags; consumed by both `lua` and
`dataentry` via arch-lint whitelist additions; renderers are pure funcs → easy
coverage; the model is the same shape a Phase-2 CalDAV server serializes.
- **Cons:** Requires `.go-arch-lint.yml` edits (new component + 2 consumer
whitelist entries); a couple of small types to keep under plimsoll caps.
- **Effort:** M.

**B2. Renderers as a Go plugin registry (`feed.Register(format, renderer)`).**
- **Pros:** Open extension.
- **Cons:** The whole codebase avoids Go-side plugin registries (registration is
config-declared, not `Register()`); renderers are a **closed set rela owns**
(correctness of iCalendar lives in Go, not in user extensions) — an open
registry is the wrong shape. Rejected.
- **Effort:** M.

**B3. iCalendar via a third-party library vs. stdlib string-building.** No iCal
lib is vendored. iCalendar is a simple line-based format; the correctness
concerns are **line folding at 75 octets, `\`-escaping of `,;\` and newlines,
CRLF line endings, stable `UID`, a `DTSTAMP`, all-day `DTSTART;VALUE=DATE`, and
a nested `VALARM`.** All are a few dozen lines of well-tested string-building.
- **Recommendation:** **stdlib string-building** — avoids a new vendor + arch-lint
vendor grant + supply-chain/govulncheck surface, and the format is small enough
to hand-roll correctly with a focused test table. Revisit a library only if
Phase-2 CalDAV needs broader iCalendar coverage (timezones, RRULE, VTODO).

### Axis C — Feed-endpoint auth (the real open question)

A feed URL is fetched by a non-browser client with no cookies/Origin, yet must
not become an unauthenticated read hole into ACL-restricted data. The endpoint
still resolves entities through the ACL **read gate**, so the question is *how
the fetching client presents a principal*.

**C1. Capability token in the URL (per-feed secret). [recommended for Phase 1]**
Each configured feed gets an opaque token; the subscription URL is
`/api/v1/_feeds/{name}.ics?token=…`. The handler is CSRF-exempt (like sync),
maps the token → a principal, and drives the read gate as that principal.
- **Pros:** Works with every calendar client (URL is all they take); no browser
context needed; naturally per-feed-scoped; the token→principal mapping is the
same seam a CalDAV `Authorization` header would later use, so Phase 2 reuses it.
Tokens live in `.rela/` (gitignored), not in `data-entry.yaml`.
- **Cons:** Token in URL can leak via logs/history — mitigate: opaque, revocable,
scoped read-only to that feed, never logged (the codebase already omits
sensitive values from logs). Needs a small token store + a `rela feed token`
issue/revoke surface.
- **Effort:** M (token store + principal mapping + revoke command).

**C2. Loopback-only, no token (rely on bind + OS).** Serve the feed only on
`127.0.0.1` and trust that local = authorized.
- **Pros:** Zero auth code; fine for a single-user localhost PIM (the stated
use case).
- **Cons:** Any local process/other local user can read the full feed; breaks
the moment the server is bound beyond loopback or put behind a proxy; doesn't
generalize to the multi-user ACL story or CalDAV. Acceptable **only** as an
explicit single-user-localhost mode, not the default design.
- **Effort:** XS.

**C3. Reuse the existing principal-header (`--principal-header`) mechanism.**
- **Pros:** No new auth; consistent with the proxy-trust model.
- **Cons:** Calendar clients can't set arbitrary headers on a subscription;
only works behind a header-injecting proxy the user controls — too niche for the
primary path.
- **Effort:** XS but rarely applicable.

## Recommendation

Build Phase 1 as **A1 + B1 + C1**, with the iCal renderer hand-rolled (B3
stdlib):

1. **`internal/feed`** — new leaf domain package (mirrors `tracer`): a
`feed.Feed`/`feed.Event`/`feed.Alarm` model and an internal closed set of
renderers (`ics`, `json`) selected by a `Format`, reading only through the
`store` interface (backend-agnostic). iCalendar hand-rolled with a
correctness-focused test table (folding, escaping, CRLF, `UID`, `DTSTAMP`,
`VALUE=DATE`, `VALARM`). No new vendor.
2. **Lua binding** — `rela.calendar.event{…}` constructors returning tagged
tables + `rela.calendar.render(events, {format=…})`, registered in
`registerReadBindings` (read-only, `ReadDeps`). Matches the `markdown.go`
functional idiom; `RaiseError` on bad input. Curation (overdue/due-today,
default surface = overdue + due-today) lives entirely in the user's script;
events deep-link via the existing `rela.url("/entity/…")`.
3. **HTTP endpoint** — `GET /api/v1/_feeds/{name}.{ics|json}` on the data-entry
server, CSRF-exempt (non-browser), driving the ACL **read gate** as the
principal resolved from a **per-feed capability token** (C1). Content type per
format. Registered like other `/api/v1/_*` system routes.
4. **Config** — `feeds: map[string]FeedConfig{Script, Format?, …}` on
`dataentryconfig.Config` (hot-reload for free), scripts under `scripts/`,
fail-fast existence check at load. Tokens stored in `.rela/` with a small
issue/revoke CLI surface.

**Tradeoffs accepted:**

- **Curation in Lua, not config** — more setup than a YAML query for the trivial
case, but the open-ended "which entities, classified how, summarized how" is
precisely what belongs in a user script; a declarative sugar layer (A3) can be
added later over the same model.
- **Hand-rolled iCalendar** — we own correctness (and its tests) instead of
importing it, trading a little code for zero new supply-chain/arch-lint surface;
acceptable because VEVENT is small. A library is revisited only if Phase-2
CalDAV demands broader coverage.
- **Token-in-URL auth** — a pragmatic, universally-client-compatible choice with
known leak-vectors mitigated by opaqueness + revocability + read-only scope +
no-logging; it is also the exact seam CalDAV auth will reuse.
- **VEVENT-only, one-way** — no checkable state round-trips; `VTODO` + CalDAV
two-way sync is deferred to Phase 2, for which this VEVENT serializer is the
reused foundation.

**Personal-glue consumers (out of core scope, noted):** the same `json` renderer
feeds a swiftbar menubar upgrade (named overdue/due-today items, click-to-open)
and a daily `launchd` macOS notification; both open `/entity/<type>/<id>`
dataentry URLs. These are personal scripts on top of the core feed, not part of
the rela feature.
