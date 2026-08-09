---
id: RR-Q2ZRSP
type: review-response
title: 'EntityToTable write/validation-path callers: automations ARE gated; only operator-boundary and write-result tables are unevaluated'
finding: The plan treats EntityToTable as the read-out Lua surface, but it has 18 call sites including write-result paths (runtime.go:1672 create_entity, :1734 update_entity), automation globals (script/executor.go:206/210 entity + old_entity, script/action.go:88), and the validation rule global (validation/lua.go:160). Those entities come from the entitymanager or raw store, NOT through Redact, so they will carry an empty/absent redacted marker regardless of policy. A script that checks is_redacted() on an automation's entity global gets a confident 'nothing redacted' that reflects the absence of a redactor, not an evaluated policy. This is the closed-world vs absent distinction DEC-T0XIWQ already had to solve for the _redacted wire field.
severity: significant
resolution: 'Partly incorrect as filed. Investigation of the wiring shows automations/cascades ARE ACL-bound: appbuild.LuaReadDepsFor''s godoc names ''automations/cascades (which run on the triggering user''s ctx)'' as an identity-bearing path, the scheduler uses the gated ScheduledLuaWriteDeps(), and data-entry automations run through App.luaWriteDeps over the gated, fail-closed scriptReader. The ungated LuaWriteDeps() has only CLI/MCP/flow callers — the operator-trust-boundary paths where ungated is deliberate and documented. So there is no automation redaction gap to fix. The residual real cases are narrow: (1) create_entity/update_entity RESULT tables, which come back from the entitymanager and are correctly unredacted — update_entity''s read is deliberately raw via WritePrepStore; (2) CLI/MCP/docs runtimes, ungated by design. Neither warrants three-way branching: on an ungated runtime EVERY entity is unevaluated, so the distinction is a property of the runtime, not of individual entities, and a script cannot act differently per-entity anyway. Resolution: drop the absent-vs-empty proposal; use a plain always-present set (empty = nothing hidden), and document that ungated runtimes (CLI, MCP, docs) always report empty.'
status: addressed
---

## Finding

`EntityToTable` has 18 call sites (`grep -rn "EntityToTable"`). They fall into
two classes the plan did not distinguish:

**Read-out (goes through `Redact` — marker is meaningful):**

- `runtime.go:981, 1001, 1040, 1117, 1629, 1997, 2030` — get/list/search
- `markdown.go:2449` — entity-refs
- `docs/module.go:108`

**NOT read-out (never touches `Redact` — marker is vacuous):**

- `runtime.go:1672` — `create_entity` result
- `runtime.go:1734` — `update_entity` result
- `script/executor.go:206` / `:210` — automation `entity` / `old_entity`
- `script/action.go:88` — action trigger entity
- `validation/lua.go:160` — validation rule `entity` global

For the second class the entity comes from the entitymanager or the raw store.
It has no redaction applied, so `entity.redacted` would be empty and
`entity:is_redacted(x)` would return false — not because the policy permits `x`,
but because nothing ever evaluated it.

## Why this matters

This is precisely the distinction DEC-T0XIWQ had to solve on the wire. Per
`redactedPropertyNames`' godoc, an empty `_redacted` list is the closed-world
"evaluated, nothing redacted" signal, and that is **deliberately distinct** from
`_redacted` being absent entirely (a shape carrying no affordances, e.g. a list
row).

The plan proposes `entity.redacted` as a set table plus an `is_redacted()`
method, with no equivalent of that absent/empty split. A script cannot tell
"policy evaluated, nothing hidden" from "no policy ran here". Given the whole
point of the ticket is letting scripts distinguish redacted from unset, shipping
an accessor that conflates evaluated-empty with never-evaluated reproduces the
original bug one level up.

## Options

1. **Absent vs empty, mirroring DEC-T0XIWQ.** Omit the `redacted` key
entirely when no redaction was evaluated; set it (possibly empty) when it was.
`is_redacted()` returns nil (not false) on an unevaluated entity, so a script
can branch three ways. Matches the established in-tree precedent.
2. **Populate on every path**, wiring a redactor into the automation and
validation runtimes. Larger blast radius, and wrong for `update_entity`'s result
— that read is deliberately raw (`WritePrepStore`), so redacting it would be the
very footgun `ReadDeps` separates the handles to prevent.
3. **Scope the accessor to read-out tables only** and document that
automation/validation globals never carry it — simplest, but the distinction is
invisible at the call site, which is how it gets misused.

Recommendation: option 1. It is the existing precedent, it is honest about what
was evaluated, and it costs one nil check in scripts that care.
