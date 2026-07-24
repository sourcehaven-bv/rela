---
id: TKT-ZF2DTV
type: ticket
title: 'lua: ReadDeps reads through visibility.Reader + visible tracer; scheduler jobs get explicit AllowAllReader; prove one role-scoped job'
kind: enhancement
priority: high
effort: l
status: ready
---

## Summary

PR 3 of the FEAT-PPH1EU arc (DEC-ZBI39P). Every Lua read becomes ACL-bound by
construction: swap the raw deps for visibility wrappers at the wiring sites.
This is the enabler for **scheduled ACL-bound LLM jobs** — a job role's reader
bounds what can enter a prompt.

## Scope

- `lua.ReadDeps`: replace `Store store.Store` with the narrow `visibility.Reader` (+ whatever minimal read surface `list_entities`/`get_relations` need — keep it consumer-side narrow); `Tracer tracer.Tracer` keeps its type but wiring injects the visibility decorator. Trace bindings (`trace_from`/`trace_to`/`find_path`) are already property-free (RES-PSZZKU) — they change behavior only via the injected decorator.
- Route the 4 property-bearing bindings through the Reader: `get_entity`, `list_entities`, `search` (re-fetches full entities per hit — must re-fetch via the Reader), `get_relations` (relation properties + peer visibility).
- Wiring sites (all pass raw deps today): `appbuild.LuaReadDeps`/`LuaWriteDeps`, `dataentry.luaWriteDeps` + validator readDeps, CLI, MCP lua_eval/lua_run, scheduler.
  - **Request paths** (data-entry actions, export_render, MCP): `PolicyReader` + visible tracer — the script reads the caller's redacted view.
  - **Scheduler**: keeps the genuine `system:*` principal + `triggered_by` stamping (already exists); receives `AllowAllReader` + raw tracer **explicitly at the wiring site** (non-regressing; capability ≠ identity). Log at startup which jobs run with an unrestricted reader.
  - **Validator**: evaluate — validation predicates evaluate against the full entity by design (RES-H5AB7S constraint); keep AllowAll there and document why.
  - **CLI**: AllowAll (operator trust boundary, RR-17DMC precedent).
- Arch-lint: add `visibility` to `lua.mayDependOn` (single entry; no cycle — RES-PSZZKU).
- NopACL byte-parity test: without acl.yaml every Lua binding's output is byte-identical to today.
- **Prove the LLM-job path end-to-end**: one scheduled job wired with a role-scoped `PolicyReader` under a `system:<job>` principal (test or example config) asserting: hidden field absent from `get_entity`/`search` results inside the script; hidden entity invisible to `list_entities`/`trace_from`; audit log attributes the job honestly.

## Non-goals

Per-job role authoring UX (follow-up). MCP tool reads (`show_entity` etc —
separate follow-up closing RES-H5AB7S's gap). Egress controls (TKT-Z1OP7R — the
other half of the LLM-job safety envelope). Write-back/derivation provenance.

## Risks / notes

- Behavior change: data-entry-invoked scripts now see the caller's redacted view; scripts assuming full reads must run under an allow-all wiring or a suitable role. Release-note it.
- The write path (`Mutator`/entitymanager) is untouched — write-prep reads stay raw below the seam, or hidden fields would be clobbered on save.
