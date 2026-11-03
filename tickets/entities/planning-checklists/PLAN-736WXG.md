---
id: PLAN-736WXG
type: planning-checklist
title: 'Planning: Extract the webhook find/create/then mapping core out of dataentry'
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
- A leaf package `internal/ingestspec` with the mapping types (`Mapping`, `Find`, `Create`, `Step`, `AppendSection`) and `Mapping.CreateType`. `dataentryconfig` keeps its `Webhook*` names as type aliases, so the YAML, JSON and every caller stay the same.
- A package `internal/ingest` with the runtime: conflict-retry loop, find, create, then-steps, compare-and-swap append, interpolation with per-value flattening, and conflict classification.
- `internal/dataentry` keeps only the HTTP parts: route registration, body reading, header allowlist, admission (load shedding), principal, status mapping, and the `body`/`query`/`header` resolver.

Out of scope:
- Moving the config validator (`validate_webhooks.go`). TKT-RUO1GY moves the mapping part when the subscription config needs it.
- Any behaviour change on the webhook endpoint.

**Acceptance Criteria:**
1. All existing webhook tests pass with no edits to their assertions (`webhook_routes_test.go`, `webhook_test.go`, `webhook_conflict_postgres_test.go`, `validate_webhooks_test.go`).
2. `internal/ingest` has its own unit tests with fake reader and writer: no-match, create, update, retry on conflict, budget exhausted, non-conflict error not retried, empty match value, missing reader.
3. Every interpolated value is flattened by `ingest`, whatever the source. A unit test in `ingest` pins it.
4. `just arch-lint` passes with the new components; `dataentryconfig` gains only the `ingestspec` edge.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: internal refactor)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: moving existing code)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal refactor)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `internal/syncmerge` is a precedent for a core shared by several write surfaces.
- Consumer-side interfaces as in `docs/architecture/consumer-side-interfaces.md`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`ingestspec` (stdlib only): the five types with their current yaml/json tags and
doc comments, plus `Mapping.CreateType()` and `Mapping.NeedsBody()`.

`ingest`:
- `Source` interface: `Resolve(ref string) string` returns the raw value of one `{{ref}}`; `DefaultMatch(prop string) string` gives the template for a match property with no `find.values` entry.
- `Interpolate(tmpl string, resolve func(string) string) string` and `FlattenToLine`, moved verbatim.
- `Deps`: `Reader` (`ListEntities`), `World store.WorldScope`, `Writer` (`CreateEntity`, `PatchEntity`), `WritePrep` (`GetEntity`, raw store), `MaxAttempts int`, `Label string` for logs. All interfaces declared in `ingest`.
- `Run(ctx, d Deps, m ingestspec.Mapping, src Source) (Result, error)` with `Result{Action, EntityID}` and the action constants. Deps are passed per call because dataentry's reader and manager are swapped on reload.
- `ErrConflictExhausted` and `IsConflict`.

`dataentry`: `webhookRouter.runPipeline` keeps admission and calls `ingest.Run`
with deps from the live accessors and `MaxAttempts: webhookMaxAttempts`.
`webhookPayload` implements `Source`; `interpolate` becomes a thin call to
`ingest.Interpolate`. `errWebhookConflictExhausted` becomes an alias of
`ingest.ErrConflictExhausted`.

**Alternatives rejected:**
- Types in `ingest` itself: `dataentryconfig` would then import entitymanager and store transitively.
- Keeping types in `dataentryconfig` and having `ingest` import it: the subscription config would depend on data-entry config.
- Copying types and converting: two definitions of one YAML shape drift.

**Files to modify:**
- new `internal/ingestspec/spec.go`, `spec_test.go`
- new `internal/ingest/ingest.go`, `interpolate.go`, `ingest_test.go`, `interpolate_test.go`
- `internal/dataentryconfig/webhook.go` (aliases)
- `internal/dataentry/webhook_routes.go`
- `.go-arch-lint.yml`, `.testcoverage.yml`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Payload values (untrusted) reach entity content only through `ingest.Interpolate`, which flattens each value. Moving flattening into `ingest` makes it apply to every future source, not only webhooks.
- The header allowlist and body cap stay in dataentry, unchanged.

**Security-Sensitive Operations:**
- Find reads go through the caller's visible reader (ACL-gated). Write-prep re-read uses the raw store, as now. Writes go through `PatchEntity` naming only changed properties.
- Error text stays unexposed: `writeWebhookError` still writes fixed messages.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. AC1: `go test ./internal/dataentry/ ./internal/dataentryconfig/` and the postgres conflict test with `RELA_TEST_DATABASE_URL`.
2. AC2/3: new table tests in `internal/ingest`.
3. AC4: `just arch-lint` in CI.

**Edge Cases:**
- Empty match value: degrade to create, never update an arbitrary entity.
- Two matches: lowest id wins.
- Create returns nil entity: error.
- Context cancelled during the retry pause.

**Negative Tests:**
- Non-unique validation error is returned at once, not retried.
- Step with no action returns an error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Live accessors captured too early after a reload. Mitigation: deps built per delivery.
- Lost flattening. Mitigation: flattening lives in `Interpolate`, pinned by tests in both packages.

Effort: m.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor ticket)

**Documentation Impact:** N/A. Internal change, no user-facing docs.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** no review-response entities; three minor points
folded into the plan:
- `Run` validates `Deps` up front: nil `Reader`, `Writer` or `WritePrep`, or `MaxAttempts < 1`, is an error before any work. Today only a nil reader is checked.
- Doc comments that link `markdown.AppendToSection` lose their brackets in `ingestspec`, which does not import markdown (doclink gate).
- `ingest` must not capture deps across calls; `Deps` is a value passed to `Run`.
