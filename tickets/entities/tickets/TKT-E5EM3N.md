---
id: TKT-E5EM3N
type: ticket
title: 'Warn on permission: values no role grants (commands, documents, navigation)'
kind: enhancement
priority: low
effort: s
status: backlog
---

## Problem

Three config surfaces accept a `permission:` naming a global ACL permission:

- `commands:` (`CommandConfig.Permission`)
- `documents:` (`DocumentConfig.Permission`, TKT-M1AX6P)
- `navigation:` (`NavigationEntry.Permission`, TKT-TXDK8U)

None of them is checked against `acl.yaml`. Any non-empty string is accepted, so
a typo (`admin:raed` for `admin:read`) is silently inert.

The failure modes are quiet and differ per surface:

- a **command** the user can never execute,
- a **document** that always 403s,
- a **nav entry** nobody can see.

The nav case is the worst of the three: a missing menu item gives no error, no
log line, and nothing to search for. Nobody notices until a user asks where
something went.

## Why it isn't already done

`internal/dataentryconfig` never sees `acl.yaml` — `ValidateConfig(data, cfg,
meta)` takes the metamodel, not the policy. Each of the three features
individually decided this was out of scope rather than plumb the policy in. That
is the right call three times over and the wrong outcome once.

## Proposal

Plumb the parsed policy (or just the set of permission names any role grants)
into config validation and emit a **warning** — not a hard error — listing
`permission:` values no role grants, naming the surface and key.

A warning rather than an error because:

- `acl.yaml` is optional; with no policy, every `permission:` is inert by
design and must not fail the boot.
- Config and policy are edited independently, sometimes by different people;
a transient mismatch during a rollout should not take the server down.

`viewCommandPermissionWarnings` (`internal/dataentryconfig/validate.go`) is the
existing precedent for a config warning of exactly this shape — reuse it.

Cover all three surfaces in one pass; fixing one and leaving two is how this
stays confusing.

## Origin

RR-2KZEXF, from the code review of TKT-TXDK8U. Documented as a gotcha in the
data-entry guide meanwhile.
