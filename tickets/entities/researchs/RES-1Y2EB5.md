---
id: RES-1Y2EB5
type: research
title: 'CalDAV/VTODO two-way sync: declarative bidirectional mapping, CalDAV↔rela identity table, and protocol surface'
summary: 'Recommend mounting a CalDAV VTODO server under /api/ in internal/dataentry (inherits the ACL read gate + JWT gate; a /caldav/ prefix fails OPEN), built on emersion/go-webdav/caldav behind a thin adapter with getctag from calfeed.CollectionTag and sync-collection deferred. MAPPING: one collection = one entity_type = one SYMMETRICAL declaration serving both directions — no sources: list, no create: block — because CalDAV natively enumerates N collections from one account URL, so an operator declares a collection per type and the user still configures the account once. Completion (STATUS/COMPLETED/PERCENT-COMPLETE) maps as one logical event. The alias service is its OWN injected leaf service (following store.VersionService) storing href→entity_id, with the rename hook as the one place old→new is knowable. AUTH IS SOLVED BY PRATIQUE, the sibling identity proxy, whose PAT class was designed FOR CalDAV by name (docs/08): HTTP Basic password-as-token, username display-only, idle-expiry not fixed expiry. rela builds no credential subsystem. NEITHER project terminates TLS — a deployment concern, and Basic over plaintext must be refused. Depends on the split-out targeted-write abstraction. DELETE maps to a status transition by default.'
status: done
---

## Problem

Phase 1 (TKT-RDM9M5, TKT-VG3NUA) ships a **read-only** calendar feed: rela
projects time-bearing entities to `VEVENT` and serves ICS/JSON at
`/api/v1/_feeds/{name}.{ics,json}`. A one-way feed cannot carry checked-off
state back, which is why RES-AHY3VS deliberately deferred `VTODO`.

Phase 2 is the two-way half: **a CalDAV server exposing rela entities as `VTODO`
collections**, so a task checked off in a native client writes back to the
graph. This is what earns `VTODO`.

### The premise is verified, not assumed

RES-AHY3VS and the wider web both suggest Apple Reminders dropped CalDAV in iOS
13 / Catalina. **That is wrong, and it was tested rather than argued.** On
2026-08-09, against a local Radicale on macOS 26.5.1: Reminders adds a
third-party CalDAV account and syncs `VTODO` **two-way**. The "dropped CalDAV"
claim refers only to *iCloud-hosted* reminders migrating to CloudKit;
third-party CalDAV account support was never removed.

This matters beyond the go/no-go: Reminders-via-CalDAV is the **only** door to
first-class native to-dos on iOS (widgets, Siri, notifications). No actively
maintained third-party iOS CalDAV task client exists; jtx Board / Tasks.org /
DAVx⁵ are Android-only, and Fantastical sources tasks only from
Reminders/Todoist/Google Tasks.

### Observed wire behaviour (ground truth for the mapping)

From that test, and from the preserved captures:

- **TLS is mandatory.** macOS `accountsd` opens with a TLS ClientHello
regardless of the scheme typed, and over plaintext HTTP it performs full
discovery but **never sends an `Authorization` header** — a 401 loop. The setup
dialog reports every failure as "account name or password verification failed"
regardless of cause.
- Agents: `accountsd` (discovery), `remindd` (sync), `dataaccessd` (calendars).
Uses `sync-collection` REPORT + `getctag` + `getetag`.
- **Completion writes three properties together**: `STATUS:COMPLETED`,
`COMPLETED:<UTC ts>`, `PERCENT-COMPLETE:100`.
- **Unknown properties survive a round-trip** — `URL`, `PRIORITY`,
`DESCRIPTION` all came back (with `URL` normalized to `URL;VALUE=URI:`). But
Reminders **rewrites the whole VCALENDAR** with its own `PRODID:-//Apple
Inc.//iOS 26.5.1//EN` and re-sorts properties, so rela must diff on **parsed
semantics, never raw bytes**.
- It **adds `DTSTART;VALUE=DATE`** mirroring `DUE`, plus `X-APPLE-SORT-ORDER`,
`CREATED`, `LAST-MODIFIED`, unprompted.
- **Client-created todos use a bare UUID** as both UID and resource filename
(`D8AAE77A-89CB-46D2-BDA4-F319D2014D6B`), not domain-qualified. rela entity IDs
must start with a letter or digit, so a raw UUID can **never** be an entity ID.
An alias table is required, not optional.
- **A client-created todo carries only `SUMMARY` + `STATUS` + timestamps** — no
due date, no priority. Any create-target type must be constructible from a title
alone.
- **Apple segregates collections by component type.** Independently confirmed
twice: Reminders bound to the `VTODO`-only collection, and Calendar.app issued
`MKCALENDAR` for a *separate* `VEVENT` collection (`displayname: Agenda`).
DAViCal's Apple wiki documents the same. A rela CalDAV collection must advertise
`supported-calendar-component-set: VTODO` **only** — mixing VEVENT and VTODO
breaks Reminders. The Phase-1 VEVENT feed stays a separate surface.

## Context

### Groundwork Phase 1 deliberately left

- `internal/calfeed` is **event-granular on purpose**. `RenderEvent` /
`RenderCollection` split so per-resource GET and whole-collection fetch share
one serializer; `ETag` (ical.go:113) and `CollectionTag` (ical.go:123) already
exist for CalDAV conditional requests, and `ETag` zeroes the clock so DTSTAMP
churn does not perturb it. `RenderEvent` hardcodes `VEVENT`, so VTODO needs a
sibling renderer plus model fields, not a rewrite.
- `feedProvider` (feed_provider.go:24) is already `List(opts.Since)` + `Get(uid)`
— its doc says a CalDAV server "additionally calls Get per resource" drives off
this single abstraction.
- `feedUID`/`splitFeedUID` (feed_uid.go) mint `<type>--<id>@rela` with a
**double**-hyphen separator chosen so hyphenated types (`test-case`) split
unambiguously. RR-NA8DML (single-hyphen ambiguity) is **already addressed** —
the code is correct today.
- RR-5880C0 documented the tombstone gap: the delta cursor advances only over
*emitted* events, so a delta consumer must derive tombstones by diffing against
the last-served ETag set.

### Auth: solved by Pratique, whose PAT class was designed FOR CalDAV

**This axis is settled by a deployment decision, not a build decision.**
`sourcehaven/pratique` is a self-hostable identity & tenant proxy that fronts an
app and injects a signed identity assertion. Its **Personal Access Token** class
exists, by name and by design, for this exact case —
`docs/08-scoped-long-lived-tokens.md` states its purpose is making "Apple
Calendar work against a self-hosted server."

What it already provides:

- **HTTP Basic, password-as-token, username display-only.**
`internal/proxy/proxy.go:219` — *"A Basic credential is only ever a PAT: its
password field is the token, its username is display-only (**a fixed rule so
CalDAV clients, which must send a username, work**)."* The doc adds: *"Do not
'validate' the username — that breaks every CalDAV client."* Machine credentials
get a 401, never a login redirect (proxy.go:188-196).
- **An `allow_basic_auth` toggle was considered and REJECTED** — "the class
exists FOR dumb clients … Bearer + Basic always both work."
- **`?token=` query params were REJECTED** as the shape behind oauth2-proxy's
CVE-2025-54576 — which independently retires the RES-1X4NS9 Option-A idea for
this surface.
- **Idle-expiry, not fixed expiry.** `core.go:371-383` — a fixed clock expiry is
"silent breakage the dumb client can't recover from." A live CalDAV poller
resets the clock on every request and never expires; a dormant client dies after
the idle TTL (default 90d). Directly relevant: a CalDAV account that stops
working on a timer is exactly the failure mode to avoid.
- **A separate low-impact capability catalog**, deliberately not sharing a
namespace with OAuth scopes ("someone *will* eventually mint a never-expiring
token against a sensitive scope"). The shipped example config
(`pratique.example.yaml:278-303`) literally uses **`calendar:read`** and
**`todo:read`** as its canonical capabilities.
- **`principal_type = pat`**, so rela can distinguish a PAT-authenticated
request from a human session and refuse it for anything sensitive.
- Per-request live membership check (free automatic offboarding), revocation,
last-used tracking, and email hygiene reminders for never-expiring tokens.
- **It mints an ES256 assertion and injects it as a trusted header**, stripping
every inbound spelling first (`proxy.go:317`, `gateAndInject` at :284) — the
CVE-2025-64484 lesson.

**rela already verifies exactly this.** `internal/jwtauth/verifier.go:1-5` is
documented as verifying "signed identity assertions (ES256 JWTs) from an
upstream OIDC identity proxy against its published JWKS," with `Issuer` /
`Audience` confused-deputy guards and a 10-minute JWKS refresh. The gate is
`requireVerifiedJWT` (jwtgate.go:128), installed via `App.SetJWTGate`, and
`-jwt-header` already defaults to **`X-Auth-Assertion`**
(cmd/rela-server/main.go:108).

Credential storage worth noting as a pattern: a 256-bit CSPRNG token stored as a
**SHA-256 hash used directly as the lookup key** — no bcrypt/argon2 anywhere in
Pratique, and no `crypto/subtle` on the PAT path, because a lookup-by-hash never
compares a stored secret against a presented one. Correct for machine
credentials; a user-chosen password would need a real KDF.

**Consequence: rela builds no credential subsystem.** The earlier finding that
rela has none stands — it simply stops being rela's problem. The CalDAV endpoint
is a JWT-gated `/api/` route like any other, and the operator issues a PAT in
Pratique's web UI (issuance is web-only by design; there is no admin CLI for it)
to paste into the Reminders password field.

### TLS: Pratique terminates it (in flight at time of writing)

**STATUS NOTE (2026-08-10):** Pratique gained in-process TLS termination on its
`feat/tls-termination` branch — cert files, or ACME/Let's Encrypt with
in-process renewal (mutually exclusive; a filled-in `auto` block without
`enabled: true` is rejected rather than silently ignored). **Not yet merged**;
confirm before relying on it. Once landed the deployment is two components —
client →TLS→ Pratique →assertion→ rela — with no separate terminator to stand
up, which is simpler than what the section below assumed. The paragraphs that
follow describe the pre-TLS state and the reasoning that produced the
deployment constraints; they are retained because the rela-side conclusions
(no TLS code, no credential subsystem, docs-only deployment work) are unchanged
either way.

### TLS, as assessed before that branch

**Correcting an earlier reading of Pratique's config:** it declares
`tls.cert_file`/`key_file`, but `cfg.TLS.CertFile` is read in exactly **one**
place — `internal/app/app.go:168`, as a boolean deciding whether to mark session
cookies `Secure`. The cert/key files are never loaded by Go code, and the
embedded Caddy sets `AutoHTTPS: Disabled` with the comment "TLS handled per
deployment; dev is plain HTTP." There is no `ListenAndServeTLS` and no autocert
in either project.

So public TLS is a **deployment-level terminator** in front of Pratique. That
still means no TLS code in rela, but the deployment story must say so
explicitly, because:

- macOS verifiably will not send Basic credentials over plaintext (the 401 loop
above), so **CalDAV without TLS simply does not work**;
- Pratique's config lets an operator set `cert_file` and silently not serve TLS
with it — a sharp edge rela's docs should not replicate.

**rela should refuse to serve Basic-authenticated CalDAV over plaintext except
in an explicit dev mode.**

Two residual rela-side items:
- **`requireLocalHost` has no `--allowed-host` flag** and no path exemption
(middleware_security.go:52-102). Behind a proxy the `Host` is whatever is
forwarded, so this needs either `--bind 0.0.0.0` (sets `allowedHosts = nil`) or
a new flag. Worth fixing properly rather than relying on the bind mode.
- **`newSecurity` hardcodes `"http://"`** when deriving `allowedOrigins`
(middleware_security.go:80-90). Latent while rela stays plaintext behind the
proxy, but a trap for anyone who later adds direct TLS.

### Other hard constraints discovered in survey

**Non-`/api/` routes fail OPEN on ACL.** `attachACLRequest` (router.go:213) and
`requireVerifiedJWT` are both gated on `isAPIPath` (`/api/` or `/api`). A
`/caldav/` prefix would get **no `acl.Request` and no read gate**, and
`readGateFromContext` returns `nopReadGate{}` = permit-all (readgate.go:167).
Mounting under `/api/` inherits every gate for free — **including the JWT gate
that carries the Pratique assertion**, which makes A1 not merely convenient but
required for the chosen auth model. There is also a documented trap
(BUG-F3ADZO): a non-`/api/` route registered on the *inner* mux is unreachable
and silently returns the SPA's 200 HTML.

**Store events are lossy by contract.** `store.Watcher` (store.go:828-834): "If
the subscriber's channel buffer is full, events are dropped." And
`pumpStoreEvents` deliberately drops the entity ID, broadcasting only a
type-scoped signal (TKT-POT9GQ — carrying the ID would make SSE an existence
oracle). So a ctag **must be content-derived**, never event-counter-derived.
`calfeed.CollectionTag` already is. fsstore has only wall-clock `UpdatedAt` (no
monotonic sequence; `rela_seq` is postgres-only), which reinforces the same
conclusion: content-hash tokens, Radicale-style.

**Writes essentially cannot fail on validation.** Per DEC-HWZHA,
`partitionValidationErrors` (entitymanager/validation.go:22) treats
required-missing, type-mismatch and invalid-value as **soft** — warnings on a
successful write, not 422s. `internal/validator` is not called from the write
path at all (custom rules are an `analyze_validations` concern). Only three hard
errors exist: `unknown_type` and `id_prefix` (both config-determined, so
config-load-checkable) and **`unique`** — a natural-key collision raised by the
entitymanager, the one genuinely dynamic hard failure.

This *strongly supports* the stated validation philosophy, with one inversion
worth naming: the risk is not runtime rejection, it is **silently creating
warning-carrying invalid entities**. A config-load check that a create-target is
constructible would be the first place in the codebase where required-ness is
fatal — a deliberate, documentable departure from DEC-HWZHA.

**Hot-reload does not validate.** `rebuildState` (watcher.go:286-300) does Load
→ Unmarshal → publish and **never calls `ValidateConfig`**; only YAML syntax
errors are caught, degrading to a `slog.Warn` and the previous config.
`ValidateConfig` is fatal only at startup (app.go:647). So load-time validation
is a strong *startup* guarantee and a weak *running* one — the CalDAV handler
cannot treat "config was validated" as an invariant.

**`state.KV` is the state seam, with a real concurrency caveat.**
`internal/state` (state.go:21-34) is an explicit swap boundary (`Get`/`Put`/
`Delete`, hierarchical keys), backed by `FSKV` over `SafeFS`, whose `WriteFile`
is a proper atomic temp→fsync→rename. But there is **no file locking and no
cross-process coordination**; every consumer works around it with an in-process
mutex plus a whole-file cache. A read-modify-write alias table is exactly the
workload that does not cover. Note `buildStateKV` is called from `assemble`, so
`state.KV` is **build-agnostic today** — all three builds get the filesystem KV,
including postgres.

**Rename capture: `EntityRenamed` is purpose-built, but the injected-hook shape
is preferred.** The alias service is its own injected service (see the
recommendation), so it may be called either from the store observer chain or
from an entitymanager hook. The observer contract below is the strongest
statement of what a rename guarantees, and applies either way.
`store.EntityObserver.EntityRenamed(oldID, renamed)` (store.go:777-804) states
that "ID-keyed observers (waiver stores, anything that stores references by
entity ID) can rewrite those references in one step," and guarantees rename
emits **exactly this one callback** — not delete+put. The entitymanager's own
comment (manager.go:816-818) is the load-bearing warning: *"Only the choke-point
knows old→new; a later sweep sees the renamed entity as an ordinary update and
cannot reconstruct this link."* A missed rename orphans the alias, and the
client sees a delete + a create — a duplicated task.

Three residual risks: observer errors are discarded at every firing site (`_ =
o.EntityRenamed(...)`); `store.Event` has **no rename op**, so a rename by
another process is indistinguishable on the event feed; and entity IDs are
case-insensitive since migration 0007, so the alias key must fold case
identically.

### Library evaluation

**`emersion/go-webdav/caldav` (MIT) is the only maintained server-capable Go
CalDAV library.** 10-method `Backend` interface; `caldav.Filter` is exported so
RFC 4791 filter matching comes free; VTODO is first-class (`server_test.go`
covers `{"VTODO"}` collections); discovery chain complete including
`/.well-known/caldav`; ETags and `If-Match` supported. Last push Jun 2026,
v0.7.0 Oct 2025, emersion still committing.

Two documented gaps to fill yourself: **server-side `sync-collection` is not
implemented** (`handleReport` dispatches only `calendar-query` and
`calendar-multiget`, else 400) and **`getctag` is absent** (zero matches). Per
sabre/dav's client guide, `sync-collection` is **optional** — clients check
`supported-report-set` and cleanly fall back to ctag/ETag polling — so the gap
degrades performance, not correctness. `getctag` is the higher-value addition
and is one property in the `calendarserver.org/ns` namespace.

The real tension: `Backend` traffics in `*ical.Calendar` (`emersion/go-ical`),
which is exactly the "don't leak parsing types" case CLAUDE.md names.
Containment is straightforward — arch-lint's `vendors:` grants are
per-component, so the dependency can be scoped to a thin adapter, keeping
`calfeed` the domain model.

Alternatives rejected: **`samedi/caldav-go` is abandoned** (last commit Jun
2019; pkg.go.dev's "updated Dec 2025" is metadata refresh, not code).
**Vikunja's CalDAV is AGPL-3.0** — excellent prior art (they hand-wrote a
244-line `sync_collection.go` on top of their samedi fork, confirming the gap),
but not vendorable. Hand-rolling everything means ~1,500 lines and running into
[golang/go#9519](https://github.com/golang/go/issues/9519) — still open — where
`encoding/xml` cannot emit prefixed elements like `<C:calendar-data>`; go-webdav
already solved this with `RawXMLValue`.

Radicale's `storage/multifilesystem/sync.py` is the clearest reference for
sync-token semantics: the token is a **content hash over present *and past* item
etags** (deleted hrefs included, which is how tombstones work), with persisted
per-token state snapshots, age-based cleanup, and an error on unknown token
forcing full re-sync.

## Options

### Axis A — Where the CalDAV server lives

**A1. Under `/api/`, inside `internal/dataentry` (recommended).** e.g.
`/api/v1/_caldav/…`, registered on the inner mux beside `_feeds`.
- **Pros:** inherits `attachACLRequest`, the read gate, **the JWT gate that
carries the Pratique assertion**, the audit principal stamp, and
`noCacheMiddleware` for free. No new security predicate. Matches the
list-table-renderer precedent. Reuses `feedEntitySource` verbatim.
- **Cons:** a slightly ugly subscription URL; couples CalDAV to the web app.
- **Effort:** M.

**A2. A sibling `/caldav/` prefix on the outer mux.**
- **Cons:** **fails open on ACL** and misses the JWT gate entirely unless
`isAPIPath` (or a deliberate second predicate) is widened — RR-T15E documents
why that predicate is intentionally narrow. Also needs the webhook-style
registration dance to avoid BUG-F3ADZO. Rejected on risk.

**A3. A new top-level `internal/caldav` package.**
- **Cons:** needs the ACL read gate and the principal, both of which live in
`dataentry`; would either import dataentry (arch-lint violation) or duplicate
the gate. A *protocol-only* leaf package with dataentry supplying the backend is
viable and is really a refinement of A1, not an alternative.

### Axis B — Protocol implementation

**B1. `emersion/go-webdav/caldav` + hand-rolled `getctag` (recommended,
accepted).** Adopt the library for the XML/protocol surface; add `getctag` from
`calfeed.CollectionTag`; skip `sync-collection` initially (clients fall back).
- **Pros:** avoids the `encoding/xml` namespace pitfall; VTODO-ready; complete
discovery; maintained; MIT. Roughly 1,500 lines not written.
- **Cons:** new vendor + arch-lint grant; `*ical.Calendar` at the boundary
needs containment.
- **Effort:** M.

**B2. Hand-roll everything.** ~1,500 lines; golang/go#9519 makes
prefixed-namespace XML genuinely painful. The Phase-1 rationale ("VEVENT is
small enough to hand-roll") does **not** transfer — CalDAV is a protocol, not a
format. Rejected.

**B3. Library + `sync-collection` up front.** The hardest single piece
(tombstones, token expiry → 507/403, persisted per-token state) and optional per
the RFC. **Deferred to a follow-up**, but design the ctag/ETag layer so it can
be added without rework.

### Axis C — Auth / transport — SETTLED

**C1. Front rela with Pratique (chosen).** Pratique resolves the CalDAV client's
HTTP Basic password as a PAT and injects an ES256 assertion that rela's existing
`jwtauth` + `requireVerifiedJWT` already verify. Public TLS is terminated by the
deployment's own terminator in front of Pratique.
- **Pros:** **zero new auth code in rela.** No credential store, no bcrypt, no
issue/revoke CLI — all of it already exists and was built with CalDAV explicitly
in mind, down to `calendar:read`/`todo:read` in the shipped example config.
Reuses the `-jwt-header` / `X-Auth-Assertion` wiring that ships today. PAT
revocation, capabilities, idle-expiry and hygiene reminders come free, and
`principal_type = pat` lets rela refuse a static credential for sensitive
operations.
- **Cons:** CalDAV requires deploying Pratique *and* a TLS terminator — a real
operational dependency for what could otherwise be a single-binary localhost
tool. The `Host` allowlist needs an `--allowed-host` story.
- **Effort:** S in rela; the work is deployment + docs.

**C2. rela-terminated TLS + its own Basic credential store.** Previously
recommended; **superseded**. It would duplicate, worse, a subsystem that already
exists next door. Keep only as the answer to "what if Pratique isn't deployed" —
and the honest answer is then "no CalDAV," since macOS verifiably will not send
credentials over plaintext.

**C3. Reuse the Phase-1 capability-token-in-URL.** CalDAV clients send
`Authorization: Basic`, not query tokens — and Pratique explicitly rejected
`?token=` as the CVE-2025-54576 shape. Rejected.

### Axis D — Deletion semantics — SETTLED

**D2. `DELETE` → a configured status transition (chosen).** Map to e.g. `status:
cancelled`, declared per collection.
- Non-destructive; the entity keeps its relations. rela has **no soft-delete
concept** (confirmed: no `archived`/`deleted_at` anywhere) and `DeleteEntity`
cascades, so a real delete from a client gesture is unacceptable as a default.
- Real delete (D1) remains available opt-in; 403 (D3) is the fallback when
neither is configured.
- Client-side confirmation is real (task apps do confirm destructive actions),
so this is defence in depth rather than the primary guard.
- The resource must then *leave* the collection, which requires the tombstone
path to be correct.

### Axis E — The mapping schema

**Revised after the multi-collection finding.** The earlier shape had
heterogeneous `sources:` plus a separate single-target `create:` block,
mirroring `FeedSource`. Multi-collection makes that unnecessary and the revision
is strictly simpler.

**One collection = one entity type = one symmetrical mapping.** A single
declaration serves both directions: read projects the entity out, write maps the
VTODO back. No `sources:` list, no `create:` block.

```yaml
caldav:
  tasks:
    meta: { name: "rela Tasks", color: "#C2185B" }
    component: vtodo            # VTODO-only collection (Apple requires this)
    entity_type: task
    where: ["status != done"]
    due: due
    summary: title
    description: notes
    completion:                 # the three-property completion event, one block
      status_property: status
      completed_value: done
      pending_value: todo
      completed_at: completed_at   # optional; receives COMPLETED
    defaults: { status: todo }  # applied to an inbound create
    on_delete: { set: { status: cancelled } }   # Axis D
  bugs:
    component: vtodo
    entity_type: bug
    where: ["status != closed"]
    due: target_date
    summary: title
    completion: { status_property: status, completed_value: closed, pending_value: open }
```

**Why this diverges from `feeds:` — the constraints genuinely differ.** ICS is
one URL per feed and read-only, so `Feed.Sources` is the only mechanism there
for combining types into one calendar (it also stands in for the OR the filter
language lacks). CalDAV is one account URL enumerating N collections: the live
test showed the client issuing `PROPFIND Depth:1` over the home-set and
discovering every collection, so an operator wanting tasks *and* bugs declares
two collections and the user still configures the account once. Multi-collection
is native, so `sources:` has no job left.

**What the single-type rule buys** is more than a smaller schema: the mapping
becomes **bidirectional by construction**. With multiple sources the read
mapping is a union while the write mapping is one branch of it, so `create:`
exists purely to re-state which branch — an asymmetry that is a symptom of the
list, not a requirement. Removing it means the create-target *is* the
collection's type: nothing to derive, nothing to disambiguate, and an inbound
`PUT` knows the type from the collection before it consults the alias service
(which therefore stores `href → entity_id`, not `→ (type, id)`).

**Accepted trade:** an interleaved mixed-type list — tasks and bugs in one
sorted list, one toggle, one colour — is not expressible. Two collections give
the same *visibility* with separate colours and independent toggling, which is
arguably the better default. If interleaving is ever wanted it is an ADDITIVE
change (`sources:` alongside `entity_type:`), not a rework.

The `completion:` block is the key departure from `FeedSource`: it maps
`STATUS`/`COMPLETED`/`PERCENT-COMPLETE` as **one logical event** in both
directions, rather than three independent property mappings. Outbound, a `status
== completed_value` entity emits all three; inbound, any of the three arriving
triggers the same transition.

**Config-load validation** (following `validateFeeds`, with the fail-fast
messages naming the exact YAML node):
- `entity_type` resolves; every named property exists on it with a compatible
type;
- `completion.completed_value` / `pending_value` are members of the status
property's enum;
- **the entity type is constructible from `SUMMARY` alone** — every required
property is either mapped, has a `defaults:` literal, or has a template default.
This is the departure from DEC-HWZHA and should be documented as deliberate;
- `component: vtodo` collections reject VEVENT-only mappings and vice versa.

**Lua escape hatch**: mutually exclusive with the declarative block, validated
with the two-arm switch `validateDocuments` uses for Command-vs-Script. Declared
example VTODOs the mapper must round-trip give testable config; note they can
only be enforced at **startup**, since hot-reload skips validation entirely.

## Recommendation

**A1 + B1 + C1 + D2 + the Axis-E schema.**

1. **Mount under `/api/`** inside `internal/dataentry` — inherits the ACL gate,
principal stamping and the JWT gate (which carries the Pratique assertion) for
free, and avoids the non-`/api/` fail-open trap.
2. **Adopt `emersion/go-webdav/caldav`** with an arch-lint vendor grant scoped
to a thin adapter; keep `calfeed` as the domain model and convert at the
boundary. Add `getctag` from `calfeed.CollectionTag` (content-derived —
mandatory, since store events are lossy and fsstore has no sequence). **Defer
`sync-collection`**; clients fall back cleanly. Add a `VTODO` renderer alongside
`RenderEvent` plus `Completed`/`PercentComplete`/`Status` on the model.
3. **Auth is a deployment concern: front rela with Pratique**, itself behind a
TLS terminator. No credential subsystem in rela. Operator issues a PAT in
Pratique's web UI and pastes it into the client's password field. rela-side
work: an `--allowed-host` story for `requireLocalHost`, a refusal to serve
Basic-authenticated CalDAV over plaintext outside an explicit dev mode, and
docs. (The latent hardcoded-`http://` origin bug should be noted but is not
triggered by this deployment.)
4. **`DELETE` → configured status transition** by default; real delete opt-in.
5. **Alias table as its OWN injected service**, in its own leaf package with its
own arch-lint component — not bolted onto an existing subsystem and not a
`store.EntityObserver` registered on the store. Follow `store.VersionService`:
an umbrella interface used **only** as the nil-able wiring vehicle threaded
through `appbuild.assemble`, with narrow consumer-side interfaces bound at each
call site (the umbrella "is a WIRING vehicle only … never a parameter to a
handler or command"). This matches the codebase's own trajectory — versioning
already moved off type-asserting the store because it is "a separate injected
concern, not a store capability" (appbuild.go:934-936). The rename hook is the
one place old→new is knowable, so the service must be called there — either via
`EntityRenamed` or, preferably, an entitymanager hook mirroring
`VersionRecorder` (version_hook.go:25), which can at least log where the
observer firing sites discard the error outright. **Note the divergence from
versioning:** version capture is explicitly best-effort, but a lost alias
silently duplicates a user's task on their phone, so whether a failed alias
write should fail the rename is a deliberate decision to record, not to inherit.
Persist via `state.KV` with **per-key** entries (not one whole-file blob) to
limit the read-modify-write clobber window — owning its own package lets the
service encapsulate that concurrency discipline once. Fold entity-ID case per
migration 0007, and follow `sync.LoadState`'s **corrupt → hard error** policy
rather than scheduler's silent-empty: a silently emptied alias table re-creates
every task as a duplicate.
6. **Depends on the targeted-write abstraction**
(`.ignored/prep-targeted-write-abstraction.md`, split to its own ticket). A
CalDAV `PUT` carries a partial entity view; applying it as a whole-entity save
would erase every property VTODO does not model — including all redacted ones.
**Do not implement inbound writes on the raw store handle.**

**Tradeoffs accepted:**

- **A vendored CalDAV library** despite calfeed's hand-rolled precedent. The
Phase-1 rationale does not transfer: VEVENT is a *format*, CalDAV is a
*protocol*, and golang/go#9519 makes the XML layer genuinely hostile. Contained
behind an adapter so `*ical.Calendar` never escapes.
- **CalDAV requires a proxied deployment.** rela alone cannot serve CalDAV to
Apple clients — macOS will not send credentials over plaintext, and building a
second credential subsystem next door to a purpose-built one is waste. This
makes CalDAV a proxied-deployment feature, not a single-binary one, and the docs
must say so plainly.
- **No `sync-collection` in v1** — sync is chattier (full ETag listings per
poll) but correct. The RFC makes it optional and clients degrade cleanly.
- **Config validation is a startup-only guarantee** — hot-reload skips
`ValidateConfig` entirely. Either accept it, or make the CalDAV handler
re-validate its own config slice on reload (a small, well-scoped fix worth
considering).
- **Deletion defaults to non-destructive**, which is *not* literal CalDAV
semantics. Justified because rela has no soft-delete to fall back on and
`DeleteEntity` cascades to relations.
- **VTODO is a flat model** — rela's graph edges have no representation.
Relations degrade to `DESCRIPTION` text or the `URL` deep link (verified to
survive the round-trip).
- **Conformance risk is real.** Vikunja's CalDAV self-describes as "early alpha,
has bugs." Budget explicit per-client testing: Reminders (macOS + iOS),
Thunderbird (VTODO native, but reported quirks about tasks landing only in
"Inbox"), eM Client, Cfait. The `.ignored/radicale-test/` rig is a working
reference to diff rela's output against.
