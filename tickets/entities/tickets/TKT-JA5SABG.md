---
id: TKT-JA5SABG
type: ticket
title: Scoped elevated updates for data-entry actions
kind: enhancement
priority: medium
effort: m
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

## Description

A data-entry action runs with the caller's rights only. `rela.bypass_acl` exists
only in automation actions (and read-only in documents), and its `admin` handle
has no `update_entity`. So an action such as OpenVWR's `indienen` cannot set a
field that users may not set directly, for example `versiestatus`. The operator
then has to give users that field, and a user can skip the action's checks
through the API (OpenVWR gap list, correction 6).

Let a data-entry action opt in to scoped elevated updates, reusing the one
escalation mechanism (`rela.bypass_acl` and its closure-scoped `admin` handle,
DEC-O59WM4):

```yaml
actions:
  indienen:
    script: indienen.lua
    permission: indienen
    allow_acl_bypass: write
    elevated_updates:
      verwerking: [versiestatus, ingediend_op]
```

- `admin.update_entity(id, properties)` exists only in such an action. It writes through `entitymanager.Manager.PatchEntity` on an elevated handle that skips the row ACL and the field gate, and only for the declared types and properties.
- The target must be readable by the caller. Elevation widens which fields the caller may write on rows they can see; it never reaches a row they cannot see.
- No content, no external refs, no other `admin` methods. Transition legality, guards and preconditions still apply.
- Audit: the `acl-bypass` row and the update row name the real principal and `triggered_by=action:<id>`.
- Load-time rules: `allow_acl_bypass` on an action must be `write`; it needs `script:`, `permission:` and a non-empty `elevated_updates:` whose types and properties are declared; `elevated_updates:` without `allow_acl_bypass: write` is refused.
- Only the `_action` endpoint and the MCP action tools (the same run path) elevate. The IdP webhook does not.

## Acceptance

- An elevated action updates a declared field the caller may not write; the same write through `rela.update_entity` is denied.
- An undeclared type or property, content, or an unreadable target is refused, and nothing is written.
- `admin.update_entity` is absent without the opt-in, and no other admin method is present.
- Audit rows carry the real principal, the bypass marker and `action:<id>`.
- Config validation refuses every invalid combination listed above.
- Docs in the data-entry guide and the Lua guide.

## Notes

A follow-up commit wraps a guide line that exceeded the markdown line limit.

A second review on the PR added the client-ceiling check, `type@face` scope
keys, the bound-row pin, value-free logs, an ACL security guide section and
corrected docs. See the review responses linked to this ticket.
