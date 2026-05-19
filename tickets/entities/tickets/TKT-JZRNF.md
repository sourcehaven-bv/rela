---
id: TKT-JZRNF
type: ticket
title: 'Data-entry SPA: render structured ACL 403 in the error UI'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Context

PR 1 of the ACL work (TKT-GN5LN) ships structured 403 responses on every write:

```json
{
  "error": "forbidden",
  "rule_kind": "read-only" | "role-grant" | "delegate-permission",
  "rule_id": "...",
  "reason": "..."
}
```

The body is on the wire (verified end-to-end with `rela-server --read-only`),
but the SPA's error toast/banner treats 403 like any other non-2xx and shows a
generic message. Operators running in `--read-only` mode see the same "something
went wrong" they'd see for a 500.

## Scope

Make the SPA recognize `error: "forbidden"` responses and render a clearer
message including `reason` (and optionally `rule_kind`/`rule_id` for power users
/ operators).

- Update the API client / error-handling helpers in `frontend/src/` to parse the structured deny body when status is 403 + JSON.
- Update toast / form-error displays to surface `reason` prominently.
- Don't gate on `rule_kind == "read-only"` specifically — the v1 ACL will produce `role-grant` and `delegate-permission` denies with the same body shape, and they should render the same way.

## Acceptance criteria

1. Hitting `rela-server --read-only` and clicking "Create entity" in any view shows a clear "Forbidden: this rela instance is configured read-only" message.
2. The toast / inline error renders the `reason` string verbatim.
3. Generic 403s (without the structured body) keep their existing "Forbidden" fallback — don't break the path for non-ACL forbids.

## Out of scope

- Hiding the create/delete buttons themselves (separate ticket — see follow-up).
- Showing a persistent banner across the whole UI (separate ticket).
- Backend changes — PR 1 already ships the wire contract.

## References

- TKT-GN5LN (ACL v0 PR 1 — ships the structured 403)
- DEC-RG878 (decision: every deny names the rule)
