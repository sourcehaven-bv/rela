---
id: PLAN-WNTU66
type: planning-checklist
title: 'Planning: Lua write bindings cannot name a face, so a faced type is uncreatable from a script'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The manager already accepts a face on both create paths
(`entity.CreateOptions.Face`, `entity.RelationOptions.FromFace`). The Lua
bindings pass empty option structs, so the value cannot be supplied. Since
`requireCreateFaceFor` makes a face *mandatory* on a faced type, this is not a
missing convenience — it is a hard dead end: no Lua script can create an entity
of a faced type at all.

**Scope (revised after design review):**

IN:

1. `rela.create_entity` accepts a face (`internal/lua/runtime.go:1710`).
2. `rela.create_relation` accepts a source face (`:1883`).
3. **`requireRelationFaceFor` in `internal/entitymanager/core.go`** — the
relation-side twin of `requireCreateFaceFor`, called from `CreateRelation` and
`UpdateRelation` *before* the ACL subject is built (RR-9LM7T7). Without it, Lua
becomes the first caller to pass an unvalidated face.
4. `admin.create_relation` (`internal/lua/elevation.go:218`) — **gated on
item 3 landing first** (RR-3NNWNK), because `authorizeAndAudit` short-circuits
under `bypassACL` (`manager.go:560-566`) and the manager check is then the only
thing validating the face.
5. Read side: `face` on `EntityToTable` (`:1328`), `from_face` on
`relationToTable` (`:1452`).

OUT, with reasons. The original plan grouped three paths under one "unverified"
note; design review resolved all three:

- **`update_entity` — the face is already carried.** `Manager.PatchEntity`
resolves via `getEntityByRef` (`manager.go:1033`), which parses the state ref
and dispatches to `GetEntityState` (`core.go:474-484`), authorizing off
`stored.Face` (`manager.go:1062`). Out of scope as code; add a confirming test
and a doc line.
- **`delete_entity` — deletes the whole FAMILY** (`manager.go:1404`,
`anyFaceOf` at `:1414`). `DeleteEntityFace` (`:1609`) is the per-face form and
`lua.Mutator` does not expose it. So `rela.delete_entity("POL-1@draft")`
resolves the ref for the type lookup and then deletes **every** face —
destructive, not merely unbuilt. **Must be documented** (RR-AQGG5O).
- **`delete_relation` — silently deletes the WRONG edge and returns true.**
`Manager.DeleteRelation` (`:2022-2024`) is hardcoded to the zero face; the
sibling doc comment (`:2029-2032`) says it "deletes the default face's, and
reports success." Filed as **BUG-YVU8CP**.
- **Worlds.** `internal/entity/writeapi.go:29-33` forbids deriving a write's
face from a world. `FEAT-CRWFACE` covers the HTTP create form.
- **MCP / CLI / caldav / provision / history-restore** — `TKT-2RQMV4`.

**Acceptance Criteria:**

1. **Faced create succeeds.** On a type declaring `faces: [draft, published]`,
`rela.create_entity("policy", {...}, "", nil, {face = "draft"})` creates a row
on `draft`; the returned table's `face` is `"draft"`.
2. **The rule is unchanged, only expressible.** The same call with no opts
table still raises `face_required`.
3. **Faceless symmetry holds.** On a faceless type, `{face = "draft"}` is
rejected (`ErrFaceNotDeclared`); no opts table behaves exactly as today.
4. **Undeclared face rejected.** `{face = "nope"}` on a faced type raises,
with the face name in the message.
5. **Malformed opts raise, never silently pass.** All of: `{face = 42}`
(wrong value type), `{fce = "draft"}` (unknown key), and **a present non-table
opts argument** such as `create_entity(t, p, c, nil, "draft")` (RR-NWBAR5).
6. **Faced relation write lands on its face.** `create_relation` on a
`scope: content` type with `{face = "draft"}` puts the edge on `draft`; a read
of the published face does not return it.
7. **Scope is enforced below the binding.** A face on an `identity`-scoped
relation type is **refused** by `requireRelationFaceFor`, from both the ordinary
and the elevated binding.
8. **No undeclared break.** Existing 3-argument calls behave identically.
**`create_relation`'s 4th argument is a deliberate, declared break**: it was
previously accepted and ignored, and now must be a table (RR-Z7BI5E). Verified
zero in-tree callers pass it; it was never in the documented signature
(`GUIDE-lua-scripting.md:351` reads three arguments).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the design question is settled by `BUG-HC6I2T` and
`BUG-64MU2Q`; the open choice was argument shape, decided below.

**Existing Solutions:**

- **No library involved.** A binding-signature change inside `internal/lua`.
- **Options-table parsing precedent — `relationQuery`, `runtime.go:1824-1854`**,
whose guard clause at `:1832` is the shape to copy: `if s.GetTop() < 1 ||
s.Get(1).Type() != lua.LTTable`. `luaUpdateEntity` uses the same
`GetTop()`-plus-type idiom at `:1767`/`:1771`.
- **`DEC-IYHLNF`** established the trailing-opts-table convention on the read
side. Extending it to writes is consistency, not a new idea.
- **The symmetric-rule precedent — `requireCreateFaceFor`
(`core.go:321-353`)** — is what makes the entity binding a dumb pipe. Item 3
gives relations the same property.
- **The scope gate the Lua path lacks** —
`internal/dataentry/relations_modern.go:301-304` zeroes the tail for identity
scope. The SPA *computes* the tail; Lua would *accept* it.
- **HTTP reference** — `internal/dataentry/write_handler.go:373-381`,
`:270-292`. Note it **diverges** on one point: `:277` trims and skips an empty
face, treating `""` as absent. Lua must not copy that (see Edge Cases).
- **Prior art:** `TKT-2RQMV4`, `BUG-64MU2Q`, `BUG-HC6I2T`, `FEAT-CRWFACE`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

```lua
rela.create_entity("policy", {title = "P"}, "body", nil, { face = "draft" })
rela.create_relation("POL-1", "cites", "FEAT-1", { face = "draft" })
```

1. **`requireRelationFaceFor(relType, fromType, face)`** in
`internal/entitymanager/core.go`, beside its entity twin. Rules: identity scope
requires the zero face; content scope on a faced source requires a declared
face; content scope on a faceless source requires zero. Called from
`CreateRelation` and `UpdateRelation` **before** the ACL subject is built, so an
unvalidated coordinate never becomes an authorization coordinate — and so the
check still runs on the `bypassACL` path.
2. **A shared opts helper** in `internal/lua`, returning
`(parsed, error)` rather than raising, so both the gated and elevated bindings
can use it (`relationQuery` at `:1830` sets this precedent). The known-key
allowlist is a **package-level var**, so the unknown-key test asserts against
the same set the parser reads.
3. **Argument handling, specified exactly** (RR-NWBAR5). Verified against
gopher-lua: `GetTop()` *does* count explicit trailing nils, so these are
distinguishable.
   - absent (`GetTop() < pos`) → no face
   - explicit `nil` → same as absent
   - a table → parse; reject unknown keys and wrong-typed values
   - **any other type present → raise**, naming the expected shape
4. **`luaCreateEntity`** (`:1710`) reads position 5 → `CreateOptions{ID, Face}`
at `:1729`. **Do not also set `newE.Face`** — `createCore` assigns `e.Face =
opts.Face` (`core.go:167`).
5. **`luaCreateRelation`** (`:1883`) reads position 4 →
`RelationOptions{FromFace, Content}` at `:1887`, which also retires the phantom
`content?` positional (a Go-comment fix only; the user docs never documented
it).
6. **`admin.create_relation`** (`elevation.go:218`) takes the same table.
7. **Read side**: `face` on `EntityToTable`, `from_face` on `relationToTable`,
**set unconditionally** (RR-QR7BH0) — `mod_time` is `""` when zero
(`:1336-1339`) and `redacted` is an empty table when empty, with a comment at
`:1347-1351` saying it is always present precisely so scripts need no nil check.
An omitted `face` would be the sole conditional key. Named constants
`argPosCreateEntityOpts = 5` / `argPosCreateRelationOpts = 4` beside
`argPosCreateEntityID` (`:38-40`), whose comment enumerates positions and must
be updated.

**Face parsing goes through `entity.ParseFace`** (`internal/entity/face.go:70`),
never an `entity.Face(s)` conversion — `face.go:10-33` documents the codec as
the sole constructor from external input.

**Alternatives considered:**

- **A 5th positional argument.** Rejected: already four positionals, and
`Prefix`/`Variant` would each need another.
- **Face on the id argument (`"POL-1@draft"`).** Rejected: on a create the id
is usually server-generated.
- **A `world` key.** Rejected on `writeapi.go:29-33`.
- **Validating scope in the Lua binding instead of the manager.** Rejected:
leaves `admin.create_relation` and every future caller (MCP, CLI, caldav — all
named in `TKT-2RQMV4`) unprotected, since the bypass path skips the ACL.
- **Silently ignoring unknown keys** (the Lua idiom). Rejected: see Security.

**Files to modify:**

- `internal/entitymanager/core.go` — `requireRelationFaceFor`
- `internal/entitymanager/manager.go` — call it in `CreateRelation` (:1831),
`UpdateRelation`
- `internal/lua/runtime.go` — `luaCreateEntity` (:1710), `luaCreateRelation`
(:1883), `EntityToTable` (:1328), `relationToTable` (:1452), opts helper, consts
(:38-40)
- `internal/lua/elevation.go` — `admin.create_relation` (:218)
- `internal/lua/runtime_test.go` — `mockManager.CreateEntity` (:165),
`.CreateRelation` (:238) must honour the face
- New `internal/lua/face_write_test.go`
- `docs-project/entities/guides/GUIDE-lua-scripting.md` (:348, :351, :431) and
`GUIDE-content-states.md` — **then `just docs`** (see Docs)

**Known call sites of `EntityToTable` outside `internal/lua`** — no change
needed, but the surface widens: `internal/docs/seed.go:121,357,420`,
`internal/script/action.go:166`, `internal/script/executor.go:225,229` (the
`entity` and `old_entity` globals), `internal/validation/lua.go:160`
(user-authored validation predicates). Also
`internal/datamigration/luastep.go:133-144` is a documented hand-copied mirror
that will **not** gain `face`; add a comment there noting the deliberate
omission, or it silently drifts.

**Dependencies:** `internal/entity` only, already imported. `lua.Mutator`
(`deps.go:115-122`) already passes both option structs — no interface change on
the Lua side. The entitymanager change is additive.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
| --- | --- | --- | --- |
| `face` string | Lua script (external to Go) | `entity.ParseFace` grammar, then `requireCreateFaceFor` / `requireRelationFaceFor` for declaration + scope | Raise; no write |
| opts table keys | same | **Allowlist** package-level var | Raise; no write |
| opts value types | same | Explicit `switch` per key | Raise; no write |
| opts argument itself | same | Must be absent, nil, or a table | Raise; no write |

**Security-Sensitive Operations:**

- **The face is an authorization coordinate.** Write grants are face-shaped
(`acl.EntitySubject.Face`, `subject.go:51`; `RelationSubject.FromFace`, `:86`),
and `GrantsVerbOnState` is exact-match — `create: ["*"]` covers only the default
face. Confirmed by review: `CreateEntity` authorizes and writes the **same**
`opts.Face` (`manager.go:748-757`), so a script naming `published` with only a
draft grant is denied. There is no widening path.
- **`BUG-Y0GNSB` is not reachable from here.** Its P1/P2 remediations are in
the code despite the `backlog` status: every `acl.EntitySubject` literal in
entitymanager names its face, enforced by `TestEveryEntitySubjectNamesItsFace`
(`facesubjectguard_test.go:55`). That bug is on the *update* path via a faced
id; this ticket touches *create*, where authorized and written faces are one
variable.
- **The elevated path is why item 3 is mandatory.** `authorizeAndAudit`
returns early under `bypassACL` (`manager.go:560-566`), so no ACL subject is
built and the ACL cannot catch a bad face. With `requireRelationFaceFor` below
the bypass, scope and declaration are still enforced. Without it,
`admin.create_relation` could write an arbitrary grammar-valid face, including
on an identity-scoped type (RR-3NNWNK).
- **A silently-dropped face is the known failure mode** — `BUG-HC6I2T`. It is
why unknown keys *and* a non-table opts argument both raise
(`write_handler.go:383-387` records the same reasoning for HTTP).
- **Read-side face needs no redaction** (RR-QW763Z): `Face` is not a property,
so `visibility.Redact` does not cover it — deliberately. The row-level gate
decides whether the script sees the row at all; if it does, the coordinate it
was fetched at discloses nothing further. Structurally the same as `type`,
already emitted. Record this reasoning in the code comment.
- **No file, crypto, or network operation** is involved.
- **Capability gating is not in play**: `capabilities.go:44-52` states
`Capabilities` covers *ambient* capabilities and is "not a confidentiality
boundary for the graph" — graph writes are gated by the ACL via `Mutator`.
`capabilityguard_test.go:51-68` guards `ai`/`http`/`write_file` only.

**Error leakage:** messages name the face and the declared set
(`sortedFaceNames`, `core.go:355`). Face names are operator-authored config and
explicitly non-confidential per CLAUDE.md. No entity content appears.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Prerequisite — the mock drops the options.** `internal/lua` has no faced
fixture, and the double ignores the fields under test:
`mockManager.CreateEntity` (`runtime_test.go:165-184`) rebuilds `newE` from
`Type`/`Properties`/`Content` and never reads `opts.Face`; `.CreateRelation`
(`:238-250`) reads `opts.Content`/`.Properties` but not `.FromFace`. **Fix the
mock first** or every test below passes vacuously.

**Two harnesses, split explicitly** — the plan previously listed all 8 ACs as if
one harness covered them; it does not:

| Harness | Covers | Why |
| --- | --- | --- |
| **Mock-level** (`mockManager`) | AC 1, 5, 6, 8 — *the binding threads the value* | Assert the `Face`/`FromFace` the mock received |
| **Integration** (real manager + faced `schema.yaml`) | AC 2, 3, 4, 7 — *the manager enforces the rule* | `requireCreateFaceFor` / `requireRelationFaceFor` live in the manager; `mockManager` has **no metamodel**, so these are unreachable through it |

Faced-schema fixtures to copy: `prototypes/worlds/project/schema.yaml:16`,
`internal/dataentry/entityref_test.go:70`.

**Assert the value, not the absence of an error.** A test checking only `err ==
nil` passes against a binding that parses the face and discards it. Every
negative must additionally assert **no write occurred** — a binding that raises
*after* calling the manager would otherwise look correct.

**Round-trip (integration).** Read a draft entity, create a sibling with `{face
= e.face}`, assert both land on `draft`. This is the case the read-side change
exists for.

**AC 6's second clause needs a fixture chain**, not a one-line mock fix: the
mock must key relations by tail, `store.RelationData` must carry it, and the
read path must filter on it. Budget for that.

**Elevated-path asymmetry**: `admin.create_relation` returns `lua.LTrue`, not a
table (`elevation.go:213-225`), so its test asserts at the mock/store boundary
only — it cannot read `from_face` off a return value.

**Edge Cases:**

| Case | Expected |
| --- | --- |
| Opts argument absent | No face; current behaviour |
| Explicit `nil` opts argument | Same as absent (`GetTop()` counts it, so this is a choice, not a limitation) |
| **Non-table opts argument** (`..., nil, "draft"`) | **Raise.** The mistake a real author makes; silently ignoring it is BUG-HC6I2T's shape |
| Empty table `{}` | Same as absent |
| `{face = ""}` | **Rejected** by `ParseFace` (`face.go:71-73`). Branch on key *presence* via `RawGetString` returning non-`LNil`, **not** on emptiness — a deliberate divergence from `write_handler.go:277`, which treats `""` as absent |
| `{face = nil}` | Same as absent |
| Face containing `@` or `--` | Rejected by `facePattern` (`face.go:59`) — both are structural separators |
| Unicode / whitespace / control chars / uppercase | Rejected by `facePattern` |
| Very long face | Accepted by grammar, rejected by the declaration check; no truncation |
| `{face = "draft"}` where `draft` is the default face | Accepted as named, not normalized to zero |
| Face on an **identity**-scoped relation | **Refused** by `requireRelationFaceFor` (AC 7) |

**Negative Tests:** wrong-typed `face`; unknown key; non-table opts argument;
undeclared face on a faced type; any face on a faceless type; face on an
identity-scoped relation. Each asserts **no write occurred**.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Mitigation |
| --- | --- | --- |
| **Face parsed then dropped** — BUG-HC6I2T's shape, silent | Medium — the mock enables it | Fix the mock first; assert at the manager boundary; integration test asserts the stored row |
| **Non-table opts silently ignored** — same shape, in the new argument | High if unspecified | Explicit raise rule + test (RR-NWBAR5) |
| **Tests that cannot fail** — mock ignores `opts`, and 4 ACs are unreachable through it | High if unaddressed | Two-harness split above |
| **Unvalidated face on relations** — no scope check exists today | Certain without item 3 | `requireRelationFaceFor` below the ACL (RR-9LM7T7) |
| **Elevated path unchecked** — `bypassACL` skips the ACL entirely | Certain if item 4 lands without item 3 | Sequencing: item 3 first (RR-3NNWNK) |
| **Docs edited in the generated tree** — guaranteed CI failure at the end of the ticket | Certain as originally planned | Edit `docs-project/`, run `just docs` (RR-NP9T1J) |
| **Undeclared break on `create_relation` arg 4** | Certain | Declared in AC 8 + release note (RR-Z7BI5E) |
| **`datamigration` mirror drifts** | Medium | Comment at `luastep.go:133` noting the deliberate omission |
| **Faced delete surprises an author** — family delete, and `delete_relation` hits the wrong edge | Medium, newly reachable | Document (RR-AQGG5O); **BUG-YVU8CP** filed |

**Effort: `s` → `m`.** The original `s` assumed `internal/lua` only. Adding
`requireRelationFaceFor` plus the two-harness fixture work crosses into
`internal/entitymanager` and is the larger half.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact** — **`docs/` is GENERATED** (RR-NP9T1J).
`justfile:423-425` runs `scripts/generate-docs.sh`, which builds `docs/` from
`docs-project/` (`:14-18`, `:35`); `docs-check` (`justfile:509-514`, part of
`just ci` at `:517`) runs `git diff --exit-code docs/ README.md docs-project/`.
Editing `docs/` directly fails CI and is reverted by the next `just docs`.

- [x] **`docs-project/entities/guides/GUIDE-lua-scripting.md`** — mutation
table `:348`/`:351`, admin table `:431`. Add the opts table and its keys, a
faced example, the declared arg-4 break, and the addressing asymmetry: a faced
id selects the face on `update_entity`, but `delete_entity` removes the whole
family and `delete_relation` targets the default face (BUG-YVU8CP).
- [x] **`docs-project/entities/guides/GUIDE-content-states.md`** — scripts name
a face directly, and why not through a world.
- [x] **Run `just docs` and commit the regenerated `docs/`.**
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`docs/data-entry.md`~~ (N/A: no UI change)
- [x] ~~`CLAUDE.md`~~ (N/A: no new pattern; follows DEC-IYHLNF)
- [x] ~~`README.md`~~ (N/A: no project-level change)

Enhancement, so a `docs-checklist` is required before `done`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings** — two reviewers (general design, security) plus
direct verification. All eight folded into the plan above:

| ID | Severity | Resolution |
| --- | --- | --- |
| RR-9LM7T7 | critical | `requireRelationFaceFor` added to scope (item 3) |
| RR-NP9T1J | critical | Docs retargeted to `docs-project/` + `just docs` |
| RR-NWBAR5 | critical | Non-table opts argument raises; AC 5 + edge case |
| RR-3NNWNK | significant | Elevated path sequenced behind item 3 |
| RR-AQGG5O | significant | Scope split per path; family-delete documented |
| RR-Z7BI5E | significant | AC 8 restated as a declared break |
| RR-QR7BH0 | minor | Read-side keys set unconditionally |
| RR-QW763Z | minor | No-redaction reasoning recorded in Security |

Spun out: **BUG-YVU8CP** (`delete_relation` silently deletes the default-face
edge and reports success) with measure `faced-write-targets-named-row-test`.
