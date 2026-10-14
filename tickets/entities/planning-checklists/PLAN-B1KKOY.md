---
id: PLAN-B1KKOY
type: planning-checklist
title: 'Planning: Feed subscriptions: poll RSS/Atom/JSON Feed into entities'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- `subscriptions.yaml` at the project root. Each entry has `url`, `every` (minimum 15m), optional `max_items` (default 50, max 1000), optional `auth`, and an ingest mapping (`find`, `create_if_missing`, `then`). References are a closed set: `item.{id,title,link,summary,content,published,published_date,updated,authors,categories}`, `feed.{title,link}`, `subscription.name`, plus `now` / `today`.
- Each subscription runs as a scheduler task named `subscription:<name>`, using the scheduler's run state, retry ladder and job queue, in both `rela scheduler` and `rela-server`.
- A run does a conditional GET with stored `ETag` / `Last-Modified`, parses with `internal/feed`, skips items already seen unchanged, converts item HTML with `htmlmd`, and writes through `ingest.Run` as `system:subscription:<name>`.
- Mapping validation shared by webhooks and subscriptions moves into `ingestspec` (RR-HEF5VV).
- `rela validate` reports `subscriptions.yaml` errors; the sqlite backend stores the file with the other root config.
- A user guide.

Out of scope: iCalendar, generic JSON API polling, UI management, hot reload
(changes apply on restart or a Configure rebuild, as for `schedules.yaml`),
`Retry-After` (the scheduler's retry ladder already backs off).

**Acceptance Criteria:**
1. A valid `subscriptions.yaml` makes the scheduler poll the feed on its `every` and create one entity per new item. A second poll of an unchanged feed performs no entity writes and no entity reads.
2. A 304 response writes nothing and keeps the stored validators.
3. An item whose content changed is processed again; an item seen unchanged is skipped; a deleted imported entity is not recreated while its item stays in the seen-set.
4. A transient failure (store, conflict exhausted, ctx) fails the run and leaves the validators unsaved. A permanent rejection (entity validation) is logged, recorded as seen, and does not fail the run.
5. An item whose identity (match values, or `item.id` without `find`) is empty is skipped with a warning and never created.
6. Item HTML reaches entities only as markdown converted by `htmlmd`; other values are flattened to one line; `item.link` and `feed.link` are resolved against the feed URL and kept only when http/https.
7. Invalid config fails `rela validate` and is logged by the server without starting subscriptions, while `schedules.yaml` tasks still start: bad name, URL, `every`, unknown reference, missing match, unknown type or property, bad auth, block content combined with `append_section`.
8. Writes are attributed to `system:subscription:<name>` in the audit log; reads go through the gated reader for that principal.
9. Redirects are followed only to the same origin (an http→https upgrade of the same host is allowed); any other redirect fails the run with a message naming the new location.
10. Existing webhook validation tests pass unchanged after the validator moves.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach settled in the ticket discussion and TKT-6KN0AC/TKT-RQ6BSA)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Parsing: `internal/feed` (TKT-6KN0AC). Mapping: `internal/ingest` (TKT-RQ6BSA).
- Scheduling: `internal/scheduler` (run state, retry ladder, run-id keyed jobs, `stampTaskAuditContext`).
- Small per-task state: `appbuild/automationjobs.go` hashes its key into `state.KV`.
- Outbound HTTP pattern: `internal/lua/http.go` (size cap, timeouts, URL check). Feed readers such as Miniflux and FreshRSS store ETag/Last-Modified per feed and follow redirects; same here.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

New package `internal/subscription`:
- `config.go`: `FileName`, `Config{Subscriptions map[string]Subscription}`, `Subscription{Description, URL, Every string, MaxItems, Auth *Auth, ingestspec.Mapping inline}`. `Parse(data, meta)` with `KnownFields`. Names use the webhook id rule. `Auth{Type: bearer|basic|header, Header, Secret, UsernameSecret}`; secrets are names in `.rela/secrets.yaml`, `token_key` refused. References are checked against the closed set at load.
- `fetch.go`: `Fetcher` with an injectable `http.RoundTripper`, 30s timeout, body read through `io.LimitReader(safexml.DefaultLimits.MaxBytes+1)` after decompression, `CheckRedirect` that allows at most 5 same-origin hops (scheme+host+port equal, or http→https on the same host with default ports), a fixed User-Agent, `Accept`, `If-None-Match` / `If-Modified-Since`. Errors carry scheme+host only, never the query.
- `source.go`: `itemSource` implements `ingest.Source`. `IsBlock(ref)` is true for `item.summary` / `item.content`, converted lazily through `htmlmd.FromHTML` (a failure yields empty and a warning). Links are resolved against the final URL and kept only when http/https.
- `seen.go`: per-subscription state in `state.KV` under `subscriptions/<sha256(name)>`: URL hash, ETag, Last-Modified (each ≤1 KiB and free of control characters, else not stored), and a seen-set of `identity hash → content hash`, capped at 4×`max_items` by last-seen order. A URL change resets the state.
- `run.go`: `Run(ctx, name, Subscription, Deps)`. Fetch; on 304 done. Parse, sort by Published then Updated, stable on document order, keep the newest `max_items` (log the drop count), process oldest first. Per item: identity from `ingest.MatchValues` (or `item.id` without `find`); empty skips; seen with the same content hash skips; else `ingest.Run`. Success or a permanent rejection (`*entitymanager.ValidationError`, `*entitymanager.ComputedWriteError`) updates the seen-set. A transient error stops the run. The seen-set is saved in every case; validators only when no transient error occurred.
- `Deps` takes a consumer-side `Host{ScheduledLuaWriteDeps() lua.WriteDeps; Store() store.Store; State() state.KV}`; nil pieces are refused.

`internal/ingest`: `Source` gains `IsBlock(ref) bool` (webhooks return false).
Only `create_if_missing.content` leaves block values unflattened.
`MatchValues(find, src)` is exported and used by `findTarget`.

`internal/ingestspec`: `Mapping.Validate(meta)` moves here from
`validate_webhooks.go` (metamodel becomes its one non-stdlib import; package doc
updated). dataentryconfig keeps the webhook-only checks and the message
substrings its tests assert.

`internal/scheduler`: `LoadConfig(ctx, loader, meta) (*Config, error)` reads
both files, parses each subscription's `every` (minimum 15m), appends one task
per subscription with an unexported-from-YAML `Subscription` field, and refuses
a task name used by both. An invalid `subscriptions.yaml` is reported but does
not drop `schedules.yaml` tasks at startup. `StartBackground`, `rela scheduler`
and `projectsetup.Validate` all call it; `meta` comes from a type-asserted
`Meta()`. A new job kind `subscription-run`; `stampTaskAuditContext` takes the
principal and `triggered_by` explicitly; the handler stamps
`system:subscription:<name>` with `ToolScheduler`.

**Alternatives rejected:**
- Running subscriptions from Lua via `schedules.yaml`: that is the escape hatch, not the declarative path the ticket asks for.
- A separate poller loop: it would duplicate run state, cross-process non-overlap and the retry ladder.
- Following cross-origin redirects with a private-address block: more code, and still leaks the URL choice to the feed publisher. A moved feed is an operator config change.
- Demoting headings in block content: fragile markdown rewriting; refusing the combination with `append_section` is simpler.

**Arch-lint:** new component `subscription` (entity, entitymanager, feed,
htmlmd, ingest, ingestspec, lua, metamodel, principal, safexml, secrets, state,
store); `ingestspec` → metamodel; `scheduler` → subscription; `projectsetup` →
scheduler (check existing). Coverage floor for `internal/subscription`.

**Files to modify:** new `internal/subscription/*`;
`internal/ingest/{ingest,interpolate}.go`;
`internal/ingestspec/{spec,validate}.go`;
`internal/dataentryconfig/validate_webhooks.go`;
`internal/dataentry/webhook_routes.go`;
`internal/scheduler/{scheduler,jobs,config}.go`;
`internal/projectsetup/validate.go`; `internal/cli/{validate,scheduler}.go`;
`internal/appbuild/projectfiles_sqlite.go`; `.go-arch-lint.yml`;
`.testcoverage.yml`; docs guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Config (operator-trusted, not editable through Configure): validated at load; URL allowlist http/https with host and no userinfo; references from a closed set.
- Feed body (untrusted): size cap after decompression, then `internal/feed` / `safexml` limits. Item HTML goes through bluemonday in `htmlmd`. Values are flattened except a block in a new entity's body, and block content cannot be combined with `append_section` (no forged section). Links are http/https only after resolution.
- Item identity (publisher-controlled): `{{subscription.name}}` lets a mapping namespace its match key; the guide recommends `[source, guid]` when several sources share a type.
- Response headers: ETag / Last-Modified stored only when short and free of control characters.

**Security-Sensitive Operations:**
- Outbound HTTP to an operator-chosen URL; no private-address block on the first hop, as the operator chose it. Redirects stay on the same origin, so the publisher cannot steer the server elsewhere and auth headers never leave the origin.
- Secrets are read by name per run, never logged. Errors and logs show scheme and host only; the state stores a URL hash.
- Writes go through the field-gated manager under `system:subscription:<name>`, so `acl.yaml` decides what a subscription may read and write. A principal that cannot read the match field sees empty identities, which are skipped rather than created; the guide says to grant read on the find type.
- State keys are hashes of the name, so any valid name maps to a valid key.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. `subscription.Run` against `httptest.Server` with a memstore-backed manager: first run creates N; second run of the same body (no validators) makes no store reads (counting store); 304 path; changed item re-processed; deleted entity not recreated.
2. Transient vs permanent: a validation rejection is recorded and the run succeeds; an injected store error fails the run and keeps validators unsaved.
3. Empty identity skipped; references outside the closed set refused at load.
4. HTML: `<script>` and headings in summary become sanitized markdown; a title with a newline is flattened; `javascript:` and relative links handled.
5. Fetch: size cap, cross-origin redirect refused, same-origin and http→https allowed, auth header present only on the origin, query string absent from errors, bad ETag not stored.
6. Config and loader: table tests for `Parse` and `scheduler.LoadConfig` (name clash, `every` below 15m, broken subscriptions keep schedules tasks); `rela validate` on a broken file.
7. Attribution: audit record carries `system:subscription:<name>`; a scheduler test drives a subscription task through the job handler.
8. Webhook validator tests run unchanged.
9. Manual: `rela-server` on a demo project polling a local feed, then a real one.

**Edge Cases:** empty feed; items without id and link; duplicate ids in one feed
(second one is seen); zero dates; `max_items` truncation keeps the newest;
410/500 status; URL change resets state; seen-set cap evicts oldest.

**Negative Tests:** unknown secret; `token_key` as secret; https→http redirect;
redirect to another host or port; too many redirects; body over the cap.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Each new item scans the find type (`ingest.findTarget`). Mitigation: the seen-set skips unchanged items before `ingest.Run`, `max_items` defaults to 50, and a counting-store test pins zero reads for an unchanged feed.
- A feed that changes item ids makes duplicates. Mitigation: the operator chooses the match key; the guide recommends `item.id` and explains its fallback to link.
- Scheduler wiring regressions. Mitigation: a test through `StartBackground`'s loader path, as the jobs.md rule asks.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** a new guide `GUIDE-feed-subscriptions` in docs-project
(generated to `docs/`), and a pointer from the scheduler and webhook guides.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** go-architect and rela-security-reviewer reviewed the
plan. All significant findings are addressed above:
- Per-item full scans and re-writes on every poll, plus deleted imports returning: seen-set with content hashes.
- Empty match keys creating duplicates: identity check and closed reference set.
- One bad item failing the task forever: transient vs permanent errors.
- Redirect auth leak and redirect SSRF: same-origin redirects only.
- Three config entry points: `scheduler.LoadConfig` shared by all three.
Minor findings adopted: minimum `every`, `Host` interface for raw store and
secrets, arch-lint list, reload stance, sort before truncate, `IsBlock` on
`Source`, relative links, `htmlmd` failure, ETag checks, hashed keys and URL,
`subscription.name`, `published_date`, shared size constant.
