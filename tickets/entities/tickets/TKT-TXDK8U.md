---
id: TKT-TXDK8U
type: ticket
title: 'Permission-based navigation filtering (UX: hide menu entries a user cannot use)'
kind: enhancement
priority: medium
effort: m
status: review
---

## Goal

Hide navigation entries a principal cannot use, **for UX reasons** — a menu full
of links that error is a bad menu. Not a security measure: see the framing
section below, which is the part most likely to be got wrong.

## Framing (read before implementing)

`docs/acl-security.md` § "Sidebar menu structure is principal-independent"
records the current behaviour and rejects per-principal hiding *as a
confidentiality tightening*:

> the metamodel is not a secret (it's served by `/api/v1/_schema`), and a
> divergent menu per principal complicates SPA caching for no confidentiality
> gain today

This ticket does **not** overturn that reasoning — it accepts it and adds a
different justification. Concretely that means:

- The endpoints keep enforcing independently. A hidden entry stays reachable
by direct URL and returns the same 403/404 it does today. Nothing about the
filter is load-bearing for authorization (same rule as `_actions` in
`internal/dataentry/CLAUDE.md`).
- Do **not** describe this as concealment in code, docs, or tests, and do not
add tests asserting that a config name is unenumerable. Config is not a secret
(root `CLAUDE.md`, "The configuration is not a secret; the data is").
- `docs/acl-security.md` must be updated so the two sections don't contradict
each other — the decision changes from "deliberately not done" to "done for UX,
explicitly not for confidentiality".

A first pass at this shipped and was reverted in TKT-M1AX6P precisely because it
was justified as concealment. The behaviour is wanted; the rationale has to be
right.

## Open design questions

1. **What is a nav entry gated on?** Only `commands:` and `documents:` have a
`permission:` field today (`config.go:611`, `:667`). Lists, kanbans and actions
have none, so most nav kinds have nothing to derive visibility from. Options:
   - add `permission:` to the nav entry itself (uniform, explicit, but
duplicates the target's own gate and can drift from it);
   - derive it from the target where one exists and add `permission:` to
`List`/`Kanban`/`Action` where it doesn't (no duplication, more surface);
   - both — derive by default, allow an explicit override on the entry.
2. **Beyond permissions?** A list whose ACL-scoped count is 0 for this
principal is arguably also a useless entry. That is a different (and more
expensive) predicate — the count already runs through the ACL read scope
(`sidebarCounts`), so it is available, but hiding on it makes the menu flicker
as data changes. Probably out of scope; decide explicitly.
3. **Empty groups.** A group whose every item is hidden should be dropped, or
it renders as a bare heading. (The reverted implementation did this.)
4. **`/_config` too, or just `/_sidebar`?** The SPA reads `SidebarData` for
the menu, so `/_sidebar` alone is enough for the UX goal. Filtering `/_config`
as well would be concealment-shaped and is explicitly not wanted.
5. **SPA caching.** The cost the existing decision names. `/api/` responses are
already `no-store` + `Vary` on the principal header (`docs/acl-security.md` §
"Caching: per-principal responses"), so this is about the SPA's own client-side
reuse of the sidebar payload, not HTTP caches. Confirm the SPA doesn't cache the
sidebar across identity changes.

## Prior art in-tree

- The reverted implementation is in git history on `feat/standalone-documents`
(`hidesNavEntry` in `internal/dataentry/views_handler.go`, removed in the revert
commit) — including the empty-group drop and a per-request predicate. Worth
reading, but re-derive the design rather than restoring it wholesale.
- `resolveCommands` (`internal/dataentry/commands.go`) is the established
pattern for "return only what this principal can use, endpoint re-checks".
- `_actions` on entity/list responses is the same shape at the item level.

## Origin

Requested by the user during TKT-M1AX6P review cleanup, after the
security-justified version was reverted. Wanted for navigation generally, not
just `document:` entries.
