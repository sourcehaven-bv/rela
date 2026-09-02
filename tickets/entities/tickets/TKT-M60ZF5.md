---
id: TKT-M60ZF5
type: ticket
title: Warn when unmatched_principal reject has no JWT gate
kind: enhancement
priority: low
effort: xs
status: done
---

## Description

`unmatched_principal: reject` only takes effect when a JWT gate is wired.
`NewRouter` snapshots `a.jwtGate` and passes `a.jwtGate != nil` to
`attachACLRequest`, which is the fact the reject decision keys on.

An operator who configures `reject` in a deployment where the JWT gate is not
(yet) wired therefore gets an `acl.yaml` key that silently does nothing — while
believing writes from unknown identities are denied. `Policy.Validate` cannot
catch it: it only checks that `principal_property` and `user_entity_type` are
set, and whether a gate is wired is a property of the SERVER, decided elsewhere
and later.

Separately, the required wiring order ("SetJWTGate MUST run before NewRouter")
is protected only by a code comment. A future refactor that reorders them breaks
the invariant quietly.

The `provision` mode already has a `slog.Warn` for its own inert state; `reject`
has nothing.

GitHub issue #1274. Source: rela#1273 (IB-review, 2026-08-06).

**Violated requirement**: CONTROL-5-15 — rules for physical and logical access
control shall be established and implemented.

**Severity**: low. Not actively exploitable; requires operator misconfiguration
or a future refactor.

## Scope

IN: a load-time warning at `NewRouter` when `unmatched_principal: reject` is
configured and no JWT gate is wired.

OUT: making it a hard error. Refusing to start would turn a wiring omission into
an outage for a deployment that is merely no stricter than the default
(`anonymous`) — the posture it had before the key was added. The operator's
mistake is believing a restriction is in force, so the fix is to say so loudly,
not to take the server down.

OUT: moving the check into `Policy.Validate`. It cannot see server wiring;
`NewRouter` is the first point where both facts are in scope.
