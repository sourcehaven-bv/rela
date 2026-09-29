---
id: BUG-BZQQDP
type: bug
title: Relation reads and writes mishandle faced endpoints
description: FilterRelations hides faced edges under ACL, relation create/GET to a faced target 404, and relation writes lack the face gate.
priority: high
effort: m
why1: Relation read and write paths looked up a relation endpoint as the zero-face row (GetEntityState(id, "")) or served every incident edge without checking which face owns it, so a faced endpoint was hidden or 404'd and a content edge travelled with a face that does not own it.
why2: Each surface (FilterRelations, the relation routes, command payloads, the gantt) did its own endpoint lookup and edge collection. None of them went through one resolver that knows faces, heads versus tails, and content scope.
why3: Faces and content-scoped relations were added after these surfaces existed. The zero-face read stayed valid on every type, and the tests seeded only faceless types, so nothing failed.
why4: There was no inventory of relation surfaces and no guard that pins zero-face reads until TKT-2528AB added the archguard allowlist and the face-awareness inventory.
why5: A default argument (the zero face, the default tail) that is correct for faceless types hides a missing address on faced ones, and each read-out surface re-implements endpoint gating instead of calling one seam.
prevention: 'Relation endpoints are gated by one batched resolver call (Resolver.EndpointsReadable / EndpointsReadableErr): head at some face, content tail at its own face. Surfaces that serve edges beside an entity apply ownedByFace. The zero-face archguard allowlist may only shrink. Stage 2 (TKT-KQXVF7) makes the tail part of every relation call.'
status: done
---

## Problem

Reported by the face-awareness inventory, not yet verified:

- `visibility.PolicyReader.FilterRelations` hides every edge touching a faced entity under an ACL (`policyreader.go:161`). This affects MCP relations and show, and Lua `get_relations`.
- Relation create to a faced target and the relation-target GET return 404 (`write_handler.go:1028`, `relation_read_handler.go:111-116`).
- Clone and relation writes lack the face gate, which gives a 403-versus-404 existence oracle.
- `DeleteRelation`, unlink and `RelationOptions{}` act on the default tail only.

## Expected

Relation reads and writes resolve faced endpoints by address and apply the face
gate with the uniform 404.

## Content-scoped edges on remaining surfaces

BUG-ISJHML (#1715) fixed the data-entry and worldreader surfaces. These may
still serve a content edge on a face that does not own it: MCP `convert.go` and
the MCP relation tools, Lua `get_relations`, and
`internal/dataentry/commands.go` (~386, ~423). Gantt also needs a check. CalDAV
is BUG-7MB1D5. Use `metamodel.IsContentScoped` and the owning-face check from
`internal/dataentry/edgeowner.go`.

## Resolution

- `FilterRelations`, relation create/GET to a faced target, and the clone and
relation-write oracle were fixed by the TKT-2528AB PRs (#1717, #1719).
- MCP show filters content edges by face (`convert.go`); MCP
`list_relations` and Lua `get_relations` query by bare id and return
`from_face`, which is correct.
- Command payloads and the gantt are fixed in the BUG-BZQQDP PR.
- The unlink and `RelationOptions{}` default tail is Stage 2 scope, moved to
BUG-J3PBFN. CalDAV stays with BUG-7MB1D5.

### Carried to TKT-KQXVF7

TKT-KQXVF7 is `ready`, so this PR does not edit it. Its PR 8 must take these
over:

- The gantt node set holds one face per id: `addGanttNodes` refuses a second
face of an id, and `ganttEdgesForType` decides content-edge ownership by the
node's face. When the gantt reads faced sources in a world, that world must
select one face per id.
- The gantt `?root=` drill declines to the full build for any faced source
type, because its closure query selects no face. Giving `collectGanttRound`
the request's world restores the fast path.
