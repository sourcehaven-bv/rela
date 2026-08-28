---
id: BUG-NRCJ9E
type: bug
title: acl audit A7 reports rela's own built-in permissions (history:read) as dead, and suggests a fix that breaks a working grant
description: 'checkDeadPermissions builds its used-set only from role_relations[].requires_permission, so the global permissions rela itself ships — acl.PermHistoryRead, acl.PermHistoryReadRedacted — are dead by construction. Their own godoc says to grant them exactly the way A7 flags. The emitted Fix (''reference it in a requires_permission gate, or remove it'') is destructive: following either branch removes a working grant or adds a meaningless write-gate.'
priority: medium
effort: s
why1: '`rela acl audit` reports [low] role "admin" grants permission "history:read" ... the permission is dead, for a permission that is live and ships with rela.'
why2: checkDeadPermissions (internal/aclaudit/tier_a.go:176) seeds its `used` map from role_relations[].requires_permission only, so any permission not consumed by a write-gate is unreferenced by construction.
why3: 'The built-in globals are not write-gates at all: PermHistoryRead gates a read path (deleted-entity history), so it can never appear in a requires_permission — the one place A7 looks.'
why4: A7 was written against the delegate-X permission family, where 'granted in permissions:' and 'consumed by requires_permission' genuinely coincide; that coincidence was encoded as a universal rule rather than a property of that one family.
why5: The audit treats acl.Policy as the complete world of permission producers AND consumers. It is complete for producers (roles[].permissions) but not for consumers, and nothing enforces that the two sets stay symmetric — so every consumer added outside acl.yaml silently turns live config into a false 'dead' report.
status: done
prevention: 'Registration over prose. BuiltinPermissions() replaces the implicit assumption that acl.Policy sees every permission consumer, and permguard_test.go scans package source so a new Perm* constant that is not registered fails CI at the moment it is written — the drift that caused this bug is now unavailable rather than merely discouraged. Note the first fix shipped with the guard MISSING: the original test iterated the registry, so an omitted constant was invisible to it, and only a code review caught that. The wider lesson: when a fix depends on a list staying in sync with source, test the SOURCE, not the list.'
---

## Symptom

A6/A7 run against a config whose only named permission is one rela itself ships.
Minimal reproduction — no `data-entry.yaml` gates at all:

```yaml
# acl.yaml
user_entity_type: persoon
principal_property: email
roles:
  everyone:
    read: [persoon]
  admin:
    permissions: ['history:read']
    read: [persoon]
```

```
$ rela --project . acl audit
[low] role "admin" grants permission "history:read" which no
      role_relations.requires_permission references; the permission is dead
      fix: reference "history:read" in a requires_permission gate, or remove it
```

`history:read` is live, and rela ships it.

## Root cause

`checkDeadPermissions` (`internal/aclaudit/tier_a.go:176`) builds its used-set
from a single source:

```go
used := map[string]bool{}
for _, def := range p.RoleRelations {
    if def.RequiresPermission != "" {
        used[def.RequiresPermission] = true
    }
}
```

`role_relations[].requires_permission` is the only permission *consumer* visible
on `acl.Policy`. Anything consumed elsewhere is dead by construction.

The built-in globals are defined in `internal/acl/policy.go`:

- `PermHistoryRead` (`policy.go:44`) — gates reading the version history of a
**deleted** entity
- `PermHistoryReadRedacted` (`policy.go:53`) — reveals fields a historical
snapshot would otherwise redact

Both godocs say they are *"granted via a role's `permissions:` list like the
delegate-X permissions"* — i.e. the code defining them documents the exact grant
shape A7 calls dead. Neither can ever appear in a `requires_permission`, because
both gate a **read** path and `requires_permission` gates relation **writes**.

## Why the Fix string is the real defect

A false positive on an advisory linter is noise. This one emits a remediation
that breaks a working deployment:

> fix: reference "history:read" in a requires_permission gate, or remove it (check for a typo)

Both branches are harmful. Removing the grant revokes a live capability; adding
a `requires_permission` puts a nonsensical write-gate on an unrelated relation
to silence the warning. An operator trusting the linter ends up worse off than
one ignoring it.

## Fix

Seed `used` with rela's built-in permissions. `internal/aclaudit` already
imports `internal/acl`, so this needs no new dependency and no interface.

Expose them as `acl.BuiltinPermissions() []string` rather than hardcoding the
pair in aclaudit — a literal list re-introduces this bug the next time a global
permission constant is added, which is precisely the why5 failure mode.

Scope note: this ticket covers **only** the built-ins, which are fixable inside
packages aclaudit already depends on. The same rule also misses
`data-entry.yaml` UI gates (documents / dashboard cards / navigation / commands)
— that needs a new consumer-side interface and a CLI adapter, and is filed
separately.

## Verification

- Reproduction above emits no A7 finding.
- A genuinely dead permission (a typo'd `delegate-foo` no gate references) is
still reported.
- Regression test asserting `acl.BuiltinPermissions()` is non-empty and that
every constant in it is A7-exempt, so a newly added global fails loudly if it is
not registered.
