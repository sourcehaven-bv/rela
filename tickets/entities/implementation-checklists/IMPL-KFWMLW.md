---
id: IMPL-KFWMLW
type: implementation-checklist
title: 'Implementation: Lua write bindings cannot name a face, so a faced type is uncreatable from a script'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**What shipped**, in the dependency order the plan required (manager check
first, so the elevated binding is never the unvalidated path):

1. `requireRelationFaceFor` — `internal/entitymanager/core.go`, called from
`CreateRelation` and `UpdateRelation` **before** the ACL subject is built. It
**rejects a wrong face and never demands one** (see RR-HQUW7V).
2. `parseWriteOpts` + the per-binding key allowlists —
`internal/lua/writeopts.go` (new).
3. `rela.create_entity` (opts at position 5), `rela.create_relation`
(position 4), `admin.create_relation` — the last gated on item 1.
4. Read side: `face` on `EntityToTable`, `from_face` and `content` on
`relationToTable`, all unconditional.
5. `internal/datamigration/luastep.go` — comment recording why its mirror of
`EntityToTable` deliberately does **not** gain `face`.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

**The prerequisite the plan called the single most important item.**
`mockManager.CreateEntity`/`CreateRelation` dropped `opts.Face`/`.FromFace`, so
tests would have passed against a binding that parsed the face and threw it
away. Both now honour it and record the face **and a call count** — the count
because the face fields are zero both when nothing was called and when a binding
called with a dropped face, which is exactly the bug under test (RR-7SJ7KN).

**Both harnesses, as planned:** `internal/lua/face_write_test.go` (binding
threads the value) and `internal/entitymanager/relationface_test.go` (manager
enforces the rule, against a real metamodel — unreachable through the mock,
which has none).

**Mutation-tested, because a test that cannot fail proves nothing.** Four
deliberate reversions, each confirmed to turn the suite red, then restored:

| Reverted | Result |
| --- | --- |
| Drop `Face:` from the `CreateOptions` literal | build failure (unused var) — weak signal, so a truer mutation was tried |
| `parseWriteOpts` default branch returns nil instead of raising | `FAIL` on both non-table cases, naming the exact scripts |
| Remove the `requireRelationFaceFor` call from `CreateRelation` | `FAIL` on both entitymanager tests |
| Re-add the `ErrFaceRequired` branch | `FAIL` on the caller-regression guard, both subtests |

The second is the defect RR-NWBAR5 predicted; the fourth is the regression
RR-HQUW7V found, now guarded.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence**

Built `cmd/rela` and ran real scripts against a throwaway project declaring
`faces: {draft, published}`, `cites` (`scope: content`) and `owned-by` (`scope:
identity`).

Happy paths — script output:

```
created face  = draft
relation face = draft
sibling face  = draft      <- round trip: create face derived from p.face
identity face = ''
```

Confirmed independently at the storage layer, which is the assertion that
matters (the binding could report a face it did not write):

```
entities/policys/POL-1@draft.md
entities/policys/POL-2@draft.md
entities/sources/SRC-1.md
relations/POL-1@draft--cites--SRC-1.md    <- content-scoped, faced
relations/POL-1--owned-by--SRC-1.md       <- identity-scoped, bare
```

Refusals, one script each:

| AC | Scenario | Result |
| --- | --- | --- |
| 2 | no face on a faced type | `this entity type declares content states; a create must name one: policy declares draft, published`; **nothing written** |
| 4 | `{face = "nope"}` | `does not declare this content state` |
| 3 | face on a faceless type | `source declares no faces, so "draft" names nothing` |
| 5 | `..., nil, "draft"` (bare string) | `options must be a table, got string` |
| 5 | `{fce = "draft"}` | `unknown option "fce"; accepted: face` |
| 7 | face on an identity-scoped relation | `relation owned-by is scope: identity, so it attaches to the entity rather than to "draft"` |

AC 8 — pre-existing shapes unchanged: 3- and 4-argument `create_entity` on a
faceless type return `face=[]`; a 3-argument `create_relation` reached the
manager (it returned `relation already exists` for an edge created earlier in
the same fixture, which is the manager answering, not the binding refusing).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: `relationQuery`'s `GetTop()`-plus-type guard and its
reject-don't-skip stance; `requireCreateFaceFor`'s error vocabulary
(`ErrFaceNotDeclared`, `sortedFaceNames`); `entity.ParseFace` as the sole
constructor from external input.

DRY: one `parseWriteOpts` serves all three bindings, and the key allowlists are
package-level vars with prebuilt lookup sets, so the unknown-key test reads the
same set the parser does. Deliberately **not** extracted:
`requireRelationFaceFor` duplicates a few lines of face-declaration checking
from `requireCreateFaceFor` rather than sharing a helper — the two now disagree
on whether a missing face is fatal, so a shared helper would need a mode flag
that obscures both.

Security: the check sits below the ACL precisely because `authorizeAndAudit`
short-circuits under `bypassACL`; pinned by
`TestCreateRelation_FaceCheckPrecedesACL`, which asserts the ACL was consulted
**zero** times for a metamodel-invalid face. The security review independently
confirmed against a live `acl.Declarative` that a script naming a face it lacks
a grant for is denied, and that both `return nil` branches are backstopped
rather than fail-open.

**Gates, all green:** `go test ./...`, `just lint` (0 issues), `just arch-lint`,
`just comment-lint`, `just plimsoll`, `just coverage-check` (79.7%), `just
docs-check` — the last being the CI failure RR-NP9T1J predicted, confirmed
avoided by editing `docs-project/` and regenerating.
