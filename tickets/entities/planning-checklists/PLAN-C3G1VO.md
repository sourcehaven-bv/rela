---
id: PLAN-C3G1VO
type: planning-checklist
title: 'Planning: lua: ReadDeps reads through visibility.Reader + visible tracer; scheduler jobs get explicit AllowAllReader; prove one role-scoped job'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Scope split (2026-07-24, user decision):** AC10 (migrate a scheduler grant into `acl.yaml` — new `FileTypeACL`, runner create-path, operator notice) and the fail-closed switch that depends on it moved to **TKT-76JP2A**. Rationale: it is its own subsystem (~8 files, a new runner capability, a consequential `NopACL`→declarative flip) and deserves independent review. Everything else below shipped in this ticket.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN (branch `feat/visibility-lua-reads` off develop): make every Lua **read-out**
ACL-bound to the acting identity; keep the write-path read raw; wire all
construction sites explicitly; add scheduler `run_as`; close the export_render
residual.

OUT: the acl.yaml migration + fail-closed switch (**TKT-76JP2A**); elevated
reads via the `admin` handle (**TKT-ACSBSA**); `list_entities` paging
(**TKT-YWDGZD**); MCP tool reads; egress controls (TKT-Z1OP7R); any
`write_access` config (rejected — DEC-O59WM4).

**Acceptance Criteria:**

1. [x] `rela.get_entity(id)` under a field policy returns the entity with hidden properties absent; a row-hidden entity returns nil.
2. [x] `rela.list_entities(type)` omits row-hidden entities and redacts survivors.
3. [x] `rela.search(q)` omits row-hidden hits (fetch-miss → `continue`) and redacts survivors.
4. [x] `rela.get_relations()` yields only relations whose BOTH endpoints are visible — including under an explicit `opts.from` (RR-7GDT1Y).
5. [x] `trace_from`/`trace_to`/`find_path` prune hidden nodes purely via the injected `VisibleTracer`, **zero binding changes**.
6. [x] **`rela.update_entity` preserves hidden properties** — write-prep read stays raw.
7. [x] **Automation/cascade scripts read as the acting user** (wired at `appbuild.go` readDeps → LuaScriptRunner).
8. [x] **Scheduled jobs read as their principal** — `system:scheduler` by default, `run_as` per task; privilege from `acl.yaml`.
9. [x] A **nil reader denies** and never falls back to the raw handle (RR-X9NVHI).
10. [→ TKT-76JP2A] Migration writes the scheduler grant into `acl.yaml`.
11. [x] NopACL byte-parity — the entire pre-existing suite passes unchanged.
12. [→ TKT-76JP2A] Role-scoped job proof end-to-end (needs the grant migration to be meaningful; the seam itself is proven by the AC1–AC5 tests, which run the real ACL + affordances engines).
13. [x] Two principals concurrently rendering the same document do not share a result — the key already carries the principal (RR-2QSGLU, shipped in TKT-L9Q669).

## Research

- [x] `/research` at arc level (RES-PSZZKU); decisions DEC-ZBI39P + DEC-O59WM4
- [x] Full seam survey taken and re-verified on develop
- [x] Existing patterns checked; reference impls consulted; concepts reviewed

**Research Doc:** RES-PSZZKU · DEC-ZBI39P · DEC-O59WM4 · builds on TKT-7I07IX
(#1194) and TKT-L9Q669 (#1188), both merged.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**As implemented:**

- `lua.EntityReader` — a 3-method local interface (`GetEntity`, `ListEntities`, `ListRelations`), the package's own call-site-interface idiom (`lua.Mutator`, `cacheStore`). `internal/lua` gained NO dependency on `visibility`; arch-lint's `lua` block is untouched.
- **`ReadDeps` split**: `VisibleReader` (read-out) + `WritePrepStore` (raw, `luaUpdateEntity` only). Names encode the boundary (RR-Y4K5E8); both godoc'd with the data-destruction hazard.
- `Runtime.reader()` — one choke point all six read sites resolve through, so the nil-deny cannot be forgotten at one of them.
- `visibility.ScriptReader` — adapts `Reader` to the 3-method surface, load-then-`Filter` so gating happens on the **stored** type (no caller type claim, no BUG-ZWTDH9 surface).
- Wiring: dataentry request paths + appbuild cascade path get policy readers + visible tracers; validator, CLI, docs runtime stay unrestricted (each with an explicit comment); scheduler gets `ScheduledLuaWriteDeps` (identity resolved per call from ctx, never bound at construction).
- Scheduler `run_as` — an identity, stamped in `stampTaskAuditContext`, so audit names the specific job.

**Deviations from plan (all deliberate):**
- `appbuild` has no affordance resolver, so its readers use `NopRedactor` — **row gating without field redaction** on scheduler/cascade paths. Documented at the call site. Data-entry (which has a resolver) gets full field redaction. Noted as the weaker-but-never-wrong half.
- Two plimsoll directives bumped by exactly the methods added (`Runtime` 119→120 for `reader()`; `Services` 21→22 for the one interface method), each with a rationale comment. The two redactor-parameterized helpers were unexported specifically to avoid a +3.
- `visibility.mayDependOn` regained `store` (RR-RT5YV8 removed it in PR 1 when only `EntityGetter` was needed; `ScriptReader` now uses `store.EntityQuery` concretely).

## Security Considerations

- [x] Input sources identified; [x] validation approach; [x] sensitive ops; [x] error handling

- The change IS the containment boundary for ACL-bound LLM jobs.
- **Write-path read stays raw by design** — guarded by test + godoc + a CLAUDE.md rule.
- Nil reader denies; gate errors drop/raise.
- **Residual, honestly stated**: script reads are permissive where no grant exists (fail-open), pending TKT-76JP2A. Also: search hit-list timing/count oracle (TKT-GGQ0JT class); `Content` body not policy-hideable; `list_entities` allocation multiplier (TKT-YWDGZD).

## Test Plan

- [x] Scenarios per AC; [x] edge cases; [x] negative tests; [x] integration approach

**Delivered** (`internal/lua/aclreads_test.go`, real `acl.Declarative` +
`affordances.PolicyResolver` over memstore, asserting on what the SCRIPT
observes):
- `TestScriptReads_HiddenFieldRedacted` (AC1), `_HiddenEntityInvisible` (AC1/2), `_RelationsPeerGated` (AC4), `_TraceGated` (AC5), `_UpdatePreservesHiddenProperties` (AC6), `_NilReaderDenies` (AC9).
- `internal/scheduler/runas_test.go`: `_RunAsOverridesIdentity`, `_EmptyRunAsKeepsSystemUser` (AC8).
- **Mutation-verified**: pointing `VisibleReader` at the raw store fails 4 tests; bypassing the tracer decorator fails the 5th. These are genuine pins, not vacuous assertions.
- Full suite green (every pre-existing Lua/dataentry/scheduler test unchanged → AC11).

## Risk Assessment

- [x] Risks + mitigations; [x] security risks; [x] effort (l)

- Data destruction — mitigated by the split + test + CLAUDE.md rule.
- Behavior change for data-entry/cascade scripts — intended; TKT-ACSBSA is the sanctioned escape hatch.
- Fail-open residual until TKT-76JP2A — explicitly tracked, not hidden.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist created when entering review

**Delivered:** `docs/transforms.md` §Access control (export_render residual →
**closed**, peer-gating + update_entity carve-out documented); CLAUDE.md "Never
redact a read that feeds a write" rule; godoc on both `ReadDeps` fields,
`EntityReader`, `ScriptReader`, `run_as`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-XC0URX (significant — cascade reads bind to
acting user; resolved with the user into DEC-O59WM4 + TKT-ACSBSA), RR-J4518A
(significant — AC6 coverage), RR-X9NVHI (significant — nil reader denies),
RR-HC42R3 (minor — allocation multiplier → TKT-YWDGZD), RR-7GDT1Y (minor —
peer-gating pinned), RR-Y4K5E8 (nit — field naming). All addressed.
