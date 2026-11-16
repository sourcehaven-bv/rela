---
id: PLAN-NSLWWN1
type: planning-checklist
title: 'Planning: Metric capture and trend charts over records'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the `rela.append_records` Lua binding on writer runtimes; the
capture pattern in the scheduled-tasks and records guides; a `display: line`
dashboard card over `/_records/{type}/_aggregate`. Out: a declarative capture
without Lua (TKT-6OYQO79).

**Acceptance Criteria:**
1. A script appends rows only when its principal may update every face of the owner (appbuild `TestRecords_LuaAppendUsesOwnerGate`).
2. The binding is absent on reader runtimes and refused inside a store transaction (lua `records_test.go`).
3. A line card config is validated at load (dataentryconfig `TestValidateRecordsCard`).
4. A line card shows nothing for an owner the reader cannot read (service `TestRecords_AggregateExcludesHiddenOwner` per_owner case; `DashboardRecordsChart.test.ts`).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the approach was agreed with Jeroen in the parent session: the generated-reports pattern)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: the chart is a small hand-written SVG; the SPA has no chart library and the gantt view is also hand-written SVG)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows rela's own attach_file binding and dashboard card patterns)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `rela.attach_file` (internal/lua) for a writer-only binding with a consumer-side interface.
- `records.Service.Append` and `Aggregate`, which already gate owners.
- `DashboardMatrix.vue` and `DashboardRatio.vue` for card components.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `lua.RecordAppender` at the consumer, an adapter in
appbuild over `records.Service.Append`, set on `WriteDeps.Records` where
`AttachFile` is set. A `records:` block on `DashboardCard` with `display: line`,
validated in dataentryconfig. The SPA component calls the aggregate API with
`per_owner=true` and draws one line per returned owner. Rejected: a server-side
card endpoint for line cards; the aggregate API already gates and folds, so a
second path would only duplicate it.

**Files to modify:** internal/lua/records.go, deps.go, runtime.go;
internal/appbuild/records.go, appbuild.go; internal/dataentry (records handler,
actions); internal/dataentryconfig; frontend dashboard view, new chart
component, records API client; guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Lua arguments: shape checked in the binding,
values by the record service. Card config: allowlists for bucket and stat,
owners capped at 8, field must be numeric.

**Security-Sensitive Operations:** Append uses the owner update check; aggregate
folds readable owners only. A stored metric is readable by anyone who reads its
owner, so the guides tell the operator to give the metric a read rule no wider
than `run_as`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:** Empty or oversized row lists; a single hidden owner (404 means
no data); a refused refetch drops the drawn line; a hidden owner keeps the other
lines' colours.

**Negative Tests:** Argument misuse raises; refusals return nil and a message;
invalid card configs fail at load.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** The value disclosure through a too-wide metric read rule is
documented, not enforced: the configuration is the operator's.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/lua-scripting.md, docs/scheduled-tasks.md,
docs/records.md, docs/data-entry.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was agreed with Jeroen in the parent session; the code review covers the implementation)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
