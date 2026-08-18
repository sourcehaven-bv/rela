---
id: BUG-919PM6
type: bug
title: acl audit A7 cannot see data-entry.yaml permission gates, so every UI-gating permission is reported dead
description: A7's used-set comes only from role_relations[].requires_permission, but data-entry.yaml gates permissions in four places (documents, dashboard cards, navigation, commands). A permission gating only UI is therefore reported dead while demonstrably working, with a Fix that would remove the grant. Needs a narrow consumer-side PermissionConsumer interface supplied by the CLI, mirroring the existing MetamodelReader pattern, plus a decision to skip A7 rather than run it blind when no consumer is injected.
priority: medium
effort: m
why1: A permission gating a standalone document, 13 dashboard cards, a nav entry and a command was reported dead three times (once per granting role) while the gating demonstrably worked.
why2: checkDeadPermissions (internal/aclaudit/tier_a.go:176) builds its used-set only from role_relations[].requires_permission, so a permission consumed by UI gates has no referencing gate to find.
why3: 'The four UI consumers live in data-entry.yaml (internal/dataentryconfig/config.go: documents :668, cards :773, navigation :896, commands :964) — a config file aclaudit is never handed.'
why4: aclaudit is deliberately bounded to internal/acl and takes the metamodel through a narrow injected MetamodelReader; no equivalent seam was defined for permission consumers, so the data-entry config had no route in.
why5: 'The audit''s dependency posture (inject narrow views of what you need) was applied to the metamodel but not carried to permissions, because A7 was assumed to be a pure-policy Tier-A check. It is not: it asserts a fact about the whole system''s consumers while seeing only one file.'
status: backlog
---

## Symptom

A permission that gates only UI surfaces is reported dead once per granting
role, while the gating works correctly:

```
[low] role "ciso" grants permission "report:sales" which no
      role_relations.requires_permission references; the permission is dead
[low] role "md" grants permission "report:sales" ...
[low] role "sales" grants permission "report:sales" ...
```

Observed against a real config where `report:sales` gates a standalone document,
13 dashboard cards, a navigation entry and a command. Verified working: a holder
gets the document (200), all cards, and the full sidebar; a non-holder gets 403,
the reduced card set, and a filtered sidebar.

Note the detection is not wrong in form — the permission genuinely has no
`requires_permission`. Only the conclusion ("dead") is false.

## Root cause

`checkDeadPermissions` (`internal/aclaudit/tier_a.go:176`) seeds `used` from
`role_relations[].requires_permission` alone. Four further consumers exist in
`internal/dataentryconfig/config.go`, none visible to `acl.Policy`:

| Surface | Field |
| --- | --- |
| documents | `Permission` (`config.go:668`) |
| dashboard cards | `Permission` (`config.go:773`) |
| navigation | `Permission` (`config.go:896`) |
| commands | `Permission` (`config.go:964`) |

As with the built-ins bug, the emitted Fix is destructive — "reference it in a
`requires_permission` gate, or remove it" either revokes a working grant or adds
a meaningless write-gate to silence the warning.

## Fix

Follow the pattern already established for the metamodel. `aclaudit` does not
import `internal/metamodel`; it declares the narrow `MetamodelReader`
(`aclaudit.go:126` — `HasEntityType` / `GetRelation` / `HasField`) plus a
`RelationView` DTO, and the concrete adapter lives consumer-side in
`internal/cli/acl.go:131`, whose comment states the intent: *"so aclaudit stays
free of a metamodel dependency and bounded to the narrow lookups the audit
actually uses."*

Add the equivalent seam:

```go
// PermissionConsumer reports permissions referenced outside acl.yaml —
// data-entry UI gates the audit cannot see from the policy alone.
type PermissionConsumer interface {
    UsedPermissions() []string
}
```

The CLI supplies the adapter, walking the loaded data-entry config for the four
fields above and returning them flattened. `aclaudit` keeps depending only on
`internal/acl`.

## Design decision: nil must NOT mean "run anyway"

There is a precedent to deliberately diverge from. The package godoc says *"A
nil MetamodelReader is valid: Audit then runs only the Tier-A checks"* — a
missing metamodel silently drops checks, which is safe because those checks
simply cannot be formed.

A7 cannot copy that. A nil `PermissionConsumer` means "I cannot see the UI
gates", and running A7 regardless reproduces this exact false positive. With no
consumer injected, A7 must **skip**, or downgrade to a "cannot verify" finding
whose message says it cannot see UI gates — never assert "dead".

Worth confirming during implementation whether every `rela acl audit` invocation
path can actually load the data-entry config. If any path cannot, that path
takes the skip branch, otherwise the bug returns through the back door.

## Rejected alternative

Scoping A7 to permissions matching the `delegate-*` family was considered as a
cheaper fix. Rejected as a destination: `delegate-*` is a naming convention, not
a checked prefix, so a renamed delegate permission would silently lose coverage.
Acceptable only as an interim patch, and only shipped together with the reworded
message.

## Verification

- A permission referenced solely by a UI gate emits no A7 finding.
- A genuinely dead permission is still reported.
- With no `PermissionConsumer` injected, A7 does not emit "dead".
- Test covering each of the four gate surfaces independently, so dropping one
from the adapter fails.

## Related

Sibling of the built-in-permissions bug (`history:read`), which is fixable
inside `internal/acl` alone and can land independently.
