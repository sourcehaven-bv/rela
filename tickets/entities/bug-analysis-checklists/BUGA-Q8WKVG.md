---
id: BUGA-Q8WKVG
type: bug-analysis-checklist
title: 'Analysis: Writes resolve their face differently from reads, so a create always lands on the bare row'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Two reproductions at the entitymanager level, both against `memstore`:

1. `CreateEntity` with `Face: "concept"` on the carrier entity returns a row at
   `face=""`. The requested coordinate is dropped silently.
2. With a face-scoped ACL double granting only `beleid@concept`, the ACL is
   asked about face `"concept"`, answers allow, and the row lands at `face=""`
   -- the one face the principal is denied. The authorization decision and the
   write target are different rows.

Reproduction 2 does NOT require `bare_face`. A type declaring `faces:` at all
exhibits it; `bare_face` only decides whether the landing row is a named face
or an unnamed one, which changes the severity and not the defect.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded as why1-why5 on BUG-HC6I2T. The sharp finding: `manager.go:736`
authorizes `acl.EntitySubject{..., Face: e.Face}` and the `createCoreOpts`
literal ten lines below carries no face at all, so the check and the write read
from different places and only one is ever populated.

`TestEveryEntitySubjectNamesItsFace` (the BUG-Y0GNSB guard) passes on this
code, because it checks that a subject literal SETS `Face`, not that the value
agrees with the write.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: thread a resolved face from the HTTP boundary to the store, so the
authorized face and the written face are the same value by construction.
`dataentry` resolves with the existing `parseEntityRef`; arch-lint forbids
`entitymanager` importing `dataentry`, so the face arrives already resolved on
`entity.CreateOptions`.

`?world=` is NOT a face input for writes. `world.go:493` refuses it on every
write on purpose: a world can answer with a FALLBACK face, so a write riding
that indirection saves the wrong state's content. Ask 1's middle input is
therefore declined; the ticket's asks 1/2/4 are met by the explicit face plus
`default_world`.

Related areas checked, from the write-path map:

- `handleV1CloneEntity` (`write_handler.go:905`) drops the source face; a clone
  of `POL-1@draft` lands bare. Same defect, fix alongside.
- `history_restore.go:145` and `cli/restore.go:68` rebuild from a snapshot with
  `entity.New`, discarding the snapshot's face.
- `ValidateCreate` (the dry-run) must thread the same face or it drifts from
  the real create, which is its entire contract.
- The create AFFORDANCE (`affordances.go:53`) must pass the same face, or
  `_actions.create` reports true for a face the principal cannot write.
- Genuinely correct as bare: `provision.go` (identity stubs have no content
  state) and the autocascade host (cascade-created entities are new families).
