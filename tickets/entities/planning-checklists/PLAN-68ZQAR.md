---
id: PLAN-68ZQAR
type: planning-checklist
title: 'Planning: Generated faces and worlds: implicit face, generated default world only without declared worlds, faced write grants must name a face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

Stage 3 is designed in the stage 3 design doc (kept in
`.ignored/stage3-design.md` on the working machine, not committed). Precedence:
§21 owner rulings, then §20 review amendments, then the main text. The work
lands as a PR sequence (§20.5).

PR 1 (this branch, `tkt-7izhp0-foundation`) is in scope now: order and names, no
behaviour change.

- `FaceOrder` and `WorldOrder` recorded from YAML, including included files (A7).
- `visibility.Family.Faces` in declaration order.
- `store.TrivialScope` / `IsTrivial`; the zero `WorldScope` is invalid, and
every zero site is set to `TrivialScope()` (A4).
- `entity.ImplicitFace`, `Face.IsImplicit()`.
- `entity.Address` with unexported fields (A10).
- `parseref` archguard guard with the full current allowlist.
- Top-level `default_world:` key in `schema.yaml`, parsed and validated (A2, D3).
`app.default_world` must match it when both are set.

Out of scope for PR 1: world generation and `Landing()` (5a), HTTP binding (5b),
ACL traversal (6), bare-id writes (7), and the rest of §20.5.

**Acceptance Criteria:**

1. A faced type's faces and the worlds come back in YAML order, also from an
included file. Test: `TestDeclOrder_FromYAML`, `TestDeclOrder_FromIncludedFile`.
2. A query with an unset `WorldScope` fails with `ErrInvalidQuery` on every
backend and naive. Test: storetest `UnsetWorldScopeIsInvalid`,
`UnsetWorldScopeOnAnEndpointIsInvalid`.
3. `ParseAddress` separates a bare id from a named face. Test: `TestParseAddress`.
4. A new `entity.ParseRef` at an edge fails the guard. Test: `TestNoNewParseRefAtEdges`.
5. `default_world:` naming an undeclared world fails the load. Test: `TestDefaultWorldKey`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: RES-Y6JA37 already covers Stage 3)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal refactor of rela types)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal refactor)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-Y6JA37

**Existing Solutions:**

- `PropertyOrder` extraction in `metamodel/loader.go` is the pattern for `FaceOrder`.
- The `FaceSelection` zero-value-invalid rule is the pattern for the `WorldScope` set flag.
- The `bareref` / `faceselect` archguard guards are the pattern for `parseref`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Design doc §2 (option C), §3.1, §17 PR 1 and §20 amendments A2, A4, A7, A10.
Alternatives A and B for the implicit face are compared in §2.1; B was rejected
because a missed store mapping hides faceless rows too.

`FaceOrderOf` / `WorldOrderOf` are free functions, not methods: `EntityDef` and
`Metamodel` are at their plimsoll exported-method lines.

**Files to modify:** `internal/store/world.go`, `faceselect.go`,
`internal/entity/{face,address}.go`,
`internal/metamodel/{types,loader,include,declorder}.go`,
`internal/visibility/resolver.go`, `internal/archguard/*`, and every zero
`WorldScope` site (dataentry, docs, mcp, lua, search, visibility, tests).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `schema.yaml` `default_world:`: must name a declared world (allowlist), or
`default` with no worlds. Anything else is a load error.
- Wire addresses: `ParseAddress` uses the existing `ParseStateRef` grammar.

**Security-Sensitive Operations:**

The unset `WorldScope` now fails closed with `ErrInvalidQuery` instead of
reading the trivial world. Every read path that had a zero scope now names
`TrivialScope()`, so read results are unchanged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria above.

**Edge Cases:** empty address, `@` only, `ID@`, NUL in id or face, multi-axis
face; an implicit-face `Ref` named through `AddressOf`; undeclared stored faces
in `Family`; a Go-built metamodel with no recorded order.

**Negative Tests:** unset scope on a query and on an endpoint predicate;
`default_world` undeclared, `default` beside declared worlds, case mismatch, set
in an included file; `app.default_world` contradicting the schema key.

Integration: full suite on fs, memorybackend, sqlite and postgres.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A missed zero `WorldScope` site now fails a read. Mitigation: a literal scan
over every `World` field, and the full suite on all four backends.
- `Family.Faces` order changes which face two fallbacks pick (relation read,
view entry). This is the declared intent of §3.1.

Effort: l for Stage 3; PR 1 is m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- `docs/data-entry.md`: one paragraph on the new schema key (via docs-project).
- PR 13 rewrites the worlds docs for the whole stage.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** recorded in the design doc: §20 lists the review
findings (G1 to G21, S-1 to S-12) and the amendments that answer them (A1 to
A19); §21 records the owner's rulings.
