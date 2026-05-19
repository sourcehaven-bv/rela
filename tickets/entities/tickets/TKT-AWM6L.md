---
id: TKT-AWM6L
type: ticket
title: Hide / disable write affordances when the server is read-only (or when an ACL denies the type)
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Context

PR 1 of the ACL work ships the wire-level deny path: writes return HTTP 403 with
a structured body. But the SPA still **shows** create / update / delete buttons
that the backend will refuse. Demo'd against `rela-server --read-only`:

- "+ Create" buttons on list views: present, click → 403 toast.
- Delete actions in detail views: present, click → 403 toast.
- Chip-picker "+ add" / inline relation editors: present, drop → 403 toast.
- Property auto-save in DynamicForm: still attempts the PATCH on every keystroke; UI feels broken because edits never persist.

Better UX: hide or disable the affordance up front and render a calm "read-only"
banner instead of letting users discover the limitation by failure.

## Scope

Two pieces:

### Backend: expose ACL mode in `/api/v1/_config`

Add a small `acl` field to the existing config endpoint:

```json
{
  "acl": {
    "mode": "open" | "read-only" | "policy",
    "writes_allowed_for": ["ticket", "concept", ...]   // optional, populated when mode == "policy"
  }
}
```

For v0:

- `mode: "open"` when the server's ACL is `acl.NopACL{}`.
- `mode: "read-only"` when the server's ACL is `acl.ReadOnlyACL{}` (i.e., `--read-only`).
- `mode: "policy"` reserved for the v1 `Declarative` ACL; v0 doesn't emit it.

`writes_allowed_for` is empty / absent for v0 — v1 will populate it from the
principal's effective roles.

### Frontend: gate affordances

When `acl.mode == "read-only"`:

- Hide or disable every "Create", "Delete", and inline-edit affordance across the SPA.
- Render a persistent banner: "This rela instance is read-only" with a brief explanation.
- DynamicForm: disable inputs, skip auto-save PATCHes (or show them as no-ops with the banner explaining why).

When `acl.mode == "open"`: behave exactly as today (no gating, no banner).

## Acceptance criteria

1. `rela-server` (no flag) → SPA shows full editing UI, no banner. Regression-test the no-ACL path.
2. `rela-server --read-only` → SPA shows banner, all write affordances hidden or disabled, no surprise 403s from the UI itself.
3. The frontend treats `acl.mode` absence as `"open"` for backwards-compat with older servers.
4. The `_config` response shape stays additive — existing fields unchanged.

## Out of scope

- Per-entity-type gating (waits for v1 `Declarative` ACL with `writes_allowed_for`).
- Per-principal customization (v1).
- MCP transport (separate ticket).

## Discussion

The `mode` enum keeps the wire contract small and extensible. Frontend gating is
a single check per affordance; no per-route conditional needed.

## References

- TKT-GN5LN (ACL v0 PR 1)
- DEC-RG878
- Will be ground truth for the v1 ticket that adds per-type `writes_allowed_for`.
