---
id: TKT-0C3II2
type: ticket
title: Reject an unmatched verified principal's writes (unmatched_principal policy key)
kind: enhancement
priority: medium
effort: s
status: in-progress
---

A cryptographically verified principal whose subject resolves to no
`user_entity_type` entity currently proceeds **anonymous but role-bearing** on
every path (keeps asserted roles; loses anything keyed on a resolved entity).
This ticket lets a **graph-is-authority** deployment opt into rejecting such a
principal's **writes** instead — the entity graph is the authority on who may
mutate, and an unknown verified identity is denied.

## Scope: reject only (provision split out)

This ticket ships the `unmatched_principal` policy key with two of its three
values: **`anonymous`** (default, today's behaviour) and **`reject`**. The third
value, **`provision`** (lazy declarative stub creation), is **deferred** — its
design is parked in `.ignored/provision-unmatched-principal-design.md` after a
design review found the write-seam genuinely unresolved (see "Why split" below).

`reject` is a pure **gate** decision — deny a write, no write of its own — so it
is small, self-contained, and high-value. `provision` is a write with a harder
seam; forcing both onto one policy key at one seam is what the review flagged.

## Why split (design-review findings)

The combined plan's review (RR-BZQ049, RR-9XBIJZ, RR-9THKDO, RR-64WDUD) found:
- The provision/reject hook as planned (per-CRUD-handler) missed sync, Lua-action,
attachment, and git-sync writes — a `reject`-bypass **and** incomplete
provision. The choke-point fix is tractable for `reject` (a gate, no write) but
hard for `provision` (needs `writeMu` + manager + ctx re-stamp + read-gate
rebuild).
- `provision`'s "automations add the groups" idea contradicted the minimal grant
(RR-64WDUD) — resolved to "bare stub only," but that plus the seam make
`provision` its own piece of work.

So: ship `reject` here at a correct gate seam; `provision` gets its own ticket
from the parked doc.

## Decided design (reject)

- **`unmatched_principal: anonymous | reject`** — a new `acl.Policy` key. Default
`anonymous` (byte-identical to today). `provision` is a **reserved** third
value: `Validate` accepts it but the data-entry path treats it as `anonymous`
with a one-time `slog.Warn("provision not yet implemented")` until its own
ticket lands — so an operator who sets it isn't silently ignored, and the key's
vocabulary is stable.
- **`reject` denies WRITES on the data-entry path.** An unmatched verified
principal's mutating request gets a generic 403; a GET stays anonymous-read
(documented posture — a graph-is-authority deployment still lets unknowns read
what their asserted roles allow; blocking reads is a separate, larger choice).
- **The seam must cover EVERY data-entry write, not just CRUD** (RR-BZQ049).
Because `reject` performs no write, it can be enforced where the write is
*authorized*, or in a shared pre-write check the CRUD + sync + action +
attachment + git paths all reach. Resolve the exact seam in planning — but a
gate has far more freedom than the provision write did (no `writeMu`/manager/
re-stamp needed).
- **Data-entry-path scoped** (RR-9THKDO / AC): fires only for a verified-JWT
unmatched principal. Guard from wiring state (`a.jwtGate != nil`), NOT from a
per-Principal marker (JWT and header principals both stamp
`Tool=ToolDataEntry`). The scheduler/header/loopback/CLI/MCP principals must be
untouched.
- **`reject` requires `principal_property` + `user_entity_type`** (load invariant)
— otherwise "unmatched" is undefined (the lookup is disabled and every request
looks unmatched). `Validate` rejects `reject` without both set.
- `isUnstamped` and the shared `ForPrincipal` gate are NOT touched — a blanket
rule there would reject the scheduler/header/loopback principals too.

## Acceptance criteria

1. `unmatched_principal` absent or `anonymous` ⇒ behaviour byte-identical to today.
2. `reject` ⇒ an unmatched verified principal's **write** (CRUD, sync, action,
attachment, git) gets 403; a matched principal's write proceeds.
3. `reject` ⇒ a **GET** by the unmatched principal is unaffected (anonymous read).
4. `reject`/the key are **data-entry-path-scoped**: scheduler, header-mode,
CLI, MCP principals are untouched (a scheduler write is not rejected).
5. `reject` set without `principal_property` + `user_entity_type` ⇒ **load error**.
6. Unknown `unmatched_principal` value ⇒ load error. `provision` is accepted at
load (reserved) but behaves as `anonymous` + a warn until its own ticket.
7. `reject`'s 403 leaks nothing (does not reveal IdP-existence vs. no-entity).
8. `isUnstamped` / shared `ForPrincipal` untouched.

## Out of scope

- **`provision`** — parked (`.ignored/provision-unmatched-principal-design.md`);
own ticket. Includes: the stub, `system:provisioner` + its migration, email
threading onto the Principal, the write-seam, the webhook race.
- Rejecting **reads** for an unmatched principal (larger posture choice).
- Org entities/relations/enforcement; `rela user import`; changing `isUnstamped`.

## Notes / prior art

- `asserted_role_assignments` (TKT-RP3X3Q) — the idiom for a new `Policy` key +
`knownPolicyKeys` + `Validate`.
- The parked design doc holds the full `provision` analysis and the four
design-review findings (RR-BZQ049/9XBIJZ/9THKDO/64WDUD).
