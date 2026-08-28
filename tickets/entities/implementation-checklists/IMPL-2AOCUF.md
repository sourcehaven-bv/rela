---
id: IMPL-2AOCUF
type: implementation-checklist
title: 'Implementation: CalDAV alias service'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`internal/caldavalias` — its own leaf package with its own arch-lint component,
wired in `appbuild.assemble` and consumed through the narrow
`entitymanager.AliasRewriter` interface. Coverage 95.7%.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Shared `newTestService` / `appleUUIDAlias` fixtures. `TestConcurrentMutations`
runs under `-race`.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The service earned its keep exactly as the ticket predicted: a to-do created in
Reminders arrives with a bare UUID (`9641DDFC-EAE6-…`) as both UID and filename,
and rela entity IDs cannot start that way, so the link must be stored. Verified
live — client-created to-dos map to new entities and survive a server restart.

**AC5 changed deliberately during the branch.** The ticket said "deleting an
entity removes its alias". It now does the opposite: the alias is RETAINED,
because an alias pointing at a missing entity is the evidence that the entity
was deleted after being served — the tombstone that lets a stale client PUT be
refused (404) instead of silently resurrecting it. `AliasRewriter.EntityDeleted`
and its call site were re-documented to match, since all three statements of the
contract had become false.

This replaced an earlier design that marked each served VTODO with
`X-RELA-ENTITY-ID` and keyed on the client echoing it back. RFC 5545 §3.8.8.2
says user agents "can ignore" x-properties, so that design failed OPEN — a
client that strips it resurrects the entity. The alias inference needs no client
cooperation at all, and works for out-of-band deletes (`rm`, `git pull`, edits
while the server is stopped) that fire no hook and no event.

Edge cases verified live: out-of-band `rm` (404, stable across retries, alias
retained), genuine create at an unseen href (201), and a corrupt table.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Two review findings landed here — RR-I4FN1T (fixed: one entity, one resource,
deterministic selection) and RR-3UAG12 (deferred: unbounded growth, with
reasoning on the entity).

Corrupt-table handling was re-sited rather than relaxed (RR-27WGOX): still fatal
where it matters — `registerCalDAVRoutes` refuses to mount — but no longer fatal
to every `rela` command on a project that has never enabled CalDAV.

`just lint` 0 issues, `just arch-lint` OK.
