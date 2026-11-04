---
id: DEC-0D8UZX3
type: decision
title: Relation endpoint types are a hard contract on every write surface
context: 'GitHub #1992: REST PATCH stored an answer-to-task edge on a relation declared answer-to-ask and answered 200 with a target_type_not_allowed warning while rela link refused the same edge. DEC-HWZHA classified type mismatches as write-with-warnings. The project owner decided the schema''s from/to types are a hard contract: there should be no broken relations and always rejecting is the simplest and most reliable rule.'
consequences: REST POST/PATCH and the per-edge relation endpoints answer 422 source_type_not_allowed or target_type_not_allowed before any write. MCP and Lua and CLI already refused and keep doing so through the same manager check. entity.RelationOptions.TolerateTypeMismatch is removed. Clients that relied on the permissive behaviour now get 422. An existing edge of a disallowed type cannot be kept in a PATCH full linkage; repair it with rela unlink.
date: "2026-11-04"
status: accepted
---

## Decision

A relation's `from` and `to` entity types are a hard contract. Every write
surface refuses an edge whose source type is not in `from` or whose target type
is not in `to`: the data-entry REST API (`POST` and `PATCH` of an entity with a
`relations` body, and the per-edge relation endpoints), the MCP tools, the Lua
bindings and the CLI.

The refusal happens before any entity or relation is written, and records no
write in the audit log. The REST API answers 422 with the stable code
`source_type_not_allowed` or `target_type_not_allowed`. The code names the end
of the canonical edge whose type is wrong, whichever name (canonical or inverse)
the request used.

There is no opt-in strict mode and no exception for edges that already exist.
The entity manager refuses every create of such an edge, so MCP, Lua and the
CLI share one check, and `entity.RelationOptions.TolerateTypeMismatch` is
removed. The data-entry API repeats the check before it writes anything, so a
PATCH or POST never changes the entity first. It also refuses a request that
names an existing disallowed edge again, so such an edge cannot be kept or
have its meta changed through that API.

## Relation to DEC-HWZHA

This supersedes DEC-HWZHA's classification of "target-type mismatch against the
relation's allowed `to` set" and "entity type not in `from` allowlist" as
write-with-warnings. The rest of DEC-HWZHA still holds: a missing target is
reported by its own rules, and unknown meta keys, unset required meta and a
declared peer type that differs from the stored one stay warnings.
