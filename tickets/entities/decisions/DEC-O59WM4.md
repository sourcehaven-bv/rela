---
id: DEC-O59WM4
type: decision
title: Script read privileges resolve through identity + acl.yaml, never inline config; scheduled jobs get run_as and a migrated grant
context: 'TKT-ZF2DTV binds Lua reads to the ACL. Two questions fell out: what do automation/cascade scripts read (they run on a user''s request ctx), and what do scheduled jobs read (they have no acting user, only system:scheduler)? Several config shapes were explored and rejected in design discussion: per-action `roles:`, a boolean `unrestricted_reads`, a four-value `...-mode` enum including write values, and a `read_access: none|principal|all` enum. Each granted privilege INLINE in task/action config, duplicating the assignment mechanism the ACL already owns and contradicting the arc''s capability-vs-identity separation (DEC-ZBI39P).'
consequences: 'Automation/cascade scripts read as the ACTING USER (symmetric with their write path). Scheduled jobs read as their principal, defaulting to system:scheduler, overridable per task via `run_as: <principal>` — an IDENTITY, not a capability. All read privilege comes from acl.yaml assignments/roles, so delegate-X tamper resistance, group expansion, local roles, `rela acl who-can` and the effective-access map all apply unchanged; there is no second grant surface to drift. Reads FAIL CLOSED: an identity with no grants reads nothing. To keep existing deployments working without relying on fail-open, a migration writes an explicit scheduler grant (role with read: ["*"] + assignment for system:scheduler) into acl.yaml, CREATING the file when absent and notifying the operator that the project is now ACL-configured (a consequential change: NopACL → declarative). No `write_access`/`unrestricted-writes` config: scheduler and automation writes both stay on entitymanager''s ACL, with rela.bypass_acl''s closure-scoped `admin` handle as the single write-escalation mechanism — config naming a capability the system lacks is worse than a missing field. Read escalation, when needed, extends that same handle with read methods (follow-up), keeping one mechanism and making privilege legible at the call site rather than ambient for a whole script.'
date: "2026-07-24"
status: accepted
---

## Decision

Script read privilege is resolved from **identity + `acl.yaml`** — never granted
inline in task or action config.

| Path | Reads as | Privilege from |
|---|---|---|
| Automation / cascade script | the **acting user** | `acl.yaml` (their existing roles) |
| Scheduled job | its principal — `system:scheduler`, or `run_as: <principal>` | `acl.yaml` assignments/roles |
| Data-entry invoked (actions, export_render, MCP) | the **request principal** | `acl.yaml` |
| CLI / docs runtime | operator trust boundary — unrestricted | n/a (RR-17DMC precedent) |

**Reads fail closed**: an identity with no grants reads nothing.

## Why not inline config

Four shapes were proposed and rejected during design, each a variant of granting
privilege where the task/action is declared:

- `roles: [...]` on an action — ambient authority for the entire script, invisible at the call site, and a **second place to grant privileges** that bypasses the ACL's delegate-X tamper resistance.
- `unrestricted_reads: true` — names the exception rather than the posture; composes badly with a second boolean.
- `...-mode: restricted | unrestricted-reads | unrestricted-writes | unrestricted` — two of the four values name a capability **the system does not have** (there is no runtime-wide write elevation; writes go through `entitymanager`). Config that names a nonexistent capability is worse than a missing field: it appears to work.
- `read_access: none | principal | all` — `all` is an inline privilege grant wearing a posture's clothing; `none`/`all` restate what a role with no grants / `read: ["*"]` already expresses. The enum invents a parallel vocabulary for a system that already has one.

`run_as` survives because it selects an **identity**, not a capability — reusing
assignments, groups, local roles, the audit log (a job logs as `system:digest`,
not a generic `system:scheduler`), and the `who-can` tooling.

## Migration

Because reads fail closed, existing deployments need an explicit grant rather
than relying on the permissive default:

```yaml
roles:
  scheduler-system:
    read: ["*"]
assignments:
  system:scheduler: scheduler-system
```

The migration **creates `acl.yaml` when absent** and **notifies the operator** —
this flips a project from `NopACL` (everything permitted) to declarative
(deny-by-default for every principal), which is consequential beyond the
scheduler and must not happen silently. Note the migration runner currently
reads-then-mutates an existing file and has no create path; that is a small
extension this work carries.

## Write escalation is unchanged

`rela.bypass_acl` remains the single mechanism: a closure-scoped object
capability, self-invalidating on exit, two-key (operator flag +
`ElevatedProvider`). Read escalation, if needed, extends that same `admin`
handle with read methods (follow-up ticket) rather than adding a config mode —
one mechanism, one mental model, privilege legible where it is exercised.

Survey and arc context: RES-PSZZKU, DEC-ZBI39P, TKT-D8T148.
