---
id: TKT-GN5LN
type: ticket
title: 'ACL v0: declarative write-side enforcement with delegate-X tamper resistance'
kind: enhancement
priority: medium
effort: l
status: done
---

## v0 scope

Ship the foundation of the authorization model: declarative roles, global
assignments, role-conferring graph relations, and the delegate-X
tamper-resistance pattern. Write-side only. No groups, no read filtering, no
inheritance, no Lua rules. See `.ignored/acl-design.md` for the full design.

## Acceptance criteria

1. **`acl.yaml` schema** at project root supports: `user_entity_type`, `roles` (with `read`, `write`, `permissions`), `assignments`, `role_relations` (with `confers` and `requires_permission`).

2. **`internal/acl` package** with an interface:

   ```go
   type ACL interface {
       AuthorizeWrite(ctx context.Context, req WriteRequest) Decision
   }
   type Decision struct {
       Allow      bool
       RuleKind   string  // role-grant | delegate-permission
       RuleID     string  // role name or permission name
       Reason     string
   }
   ```

3. **Wired into `entitymanager.Deps`** as a required collaborator (constructor rejects nil per CLAUDE.md). `NopACL` (allow-all) is the explicit opt-out for projects without `acl.yaml`.

4. **Write enforcement**: every `Manager.{Create,Update,Delete}{Entity,Relation}` consults the ACL before persisting. On deny, returns a typed error that the data-entry HTTP handler maps to **HTTP 403 with structured body**:

   ```json
   {
     "error": "forbidden",
     "rule_kind": "...",
     "rule_id": "...",
     "reason": "..."
   }
   ```

5. **Delegate-X tamper resistance**: writing a role-relation (e.g. `alice --editor-of--> TKT-001`) requires the writer to hold `delegate-<role>` permission. Admins hold all `delegate-*`; lesser roles can hold a subset (graduated trust).

6. **Audit interaction**: denied writes are recorded by the audit log (forensic event). New audit op: `denied-write`. Reuses the existing `Principal` attribution.

7. **Snapshot semantics**: principal's effective roles are computed once per request and cached on context. No long-lived cache.

8. **Missing `acl.yaml` behavior**: allow everything; emit a startup `slog.Warn` when the server binds non-loopback AND `acl.yaml` is absent.

9. **Error contract**: every deny response names the rule that fired (per AWS IAM lesson — opaque denials are unsupportable).

10. **Tests** include:
    - Allow path: principal with role grants writes.
    - Deny path: principal without role gets structured 403.
    - Delegate-X: editor cannot create an admin role-binding; admin can.
    - Audit: denied writes produce `denied-write` audit rows.
    - Missing `acl.yaml`: backwards-compatible allow-all.

## Out of scope (deferred to v1+)

- Read filtering, property redaction, list responses with `filtered_count` (v1).
- Groups and `member-of` transitive resolution (v1).
- MCP transport intersection (v1).
- Containment inheritance, `except_properties`, explicit-deny rules (v2).
- Documented automation patterns for time-based / segregation-of-duties rules (v3).

## Prototype validation (Python, SQLite)

A throwaway Python prototype in `.ignored/acl-prototype/` exercised the full
v0+v1 design end-to-end. Four scenarios — tickets, ISMS, PM tool, and a
per-entity local-role filter — all run as expected. Key findings:

- **Store-neutral query DSL**: a small `Query{HasInbound|HasOutbound: RelationFilter{Endpoints, OfTypes, InheritThrough, Depth}}` shape carries graph-shape questions to the store with no ACL jargon. The DSL is reusable for watch lists, containment traversal, "documents linking to concept X," etc.
- **Property redaction stays out of the DSL**: post-query loop in the ACL layer. Simpler contract, identical across backends.
- **No N+1**: in scenario 4 (1000 tickets, 142 visible via `assigned-to` local role), the SQLite backend filters in a single `EXISTS` subquery — 4 queries total for the entire request, not 1000+ lookups.
- **Delegate-X tamper resistance** works as designed: editors cannot create admin role-bindings; admins can. Verified on bob (lacking `delegate-contributor`) → 403, and jeroen → 200.
- **Group expansion via recursive CTE** handles arbitrary depth in one query (cap at 5 per design).
- **Per-request query budget**: write check = 1 query; list with global read = 3–4 (could be 2–3 with the snapshot-once optimization from CLAUDE.md); list with per-entity local-role = same 3–4.

The prototype validates that the design is implementable in a few hundred lines
and the perf claims hold. v0 in Go will be a direct port of the write-side
authorize logic + audit-log integration.

## References

- Design doc: `.ignored/acl-design.md`
- Prototype: `.ignored/acl-prototype/` (store.py, acl.py, scenarios.py)
- Decision: DEC-RG878
- Feature: FEAT-AESD4
- Builds on: TKT-WEBI (Principal plumbing, PR #767), FEAT-831A (audit-log).
