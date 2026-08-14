---
id: PLAN-9FGKSD
type: planning-checklist
title: 'Planning: Unified targeted-write primitive: entitymanager.PatchEntity replaces four hand-rolled property merges'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Revised after design review (7 findings, all addressed).** The ordering in
> §3 and the scope of the `IsLocked` guard changed materially. See "Design
> Review" at the bottom.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** four writers hand-roll the same read-modify-write with four
incompatible clear/body dialects (table in TKT-80EWGM). No shared helper exists:
`internal/entity` has `GetString`/`SetString`/`Clone` and nothing else. Each new
writer reinvents it and must independently rediscover the read-out/write-prep
boundary (DEC-ZBI39P), guarded today by prose in four places.

**Scope:**

IN:
- `PatchEntity` on `*Manager` + `entity.EntityPatch`.
- Extract `updateCore` so `UpdateEntity` and `PatchEntity` share one pipeline.
- Field-write gate as a **required** injected `Deps` capability.
- **`IsLocked()` git-crypt guard** — moved IN by RR-0QWLRC.
- Migrate `internal/lua` `luaUpdateEntity`, `internal/mcp` `update_entity`,
`internal/cli` `update`.
- Delete `lua.ReadDeps.WritePrepStore`.

OUT (and why):
- **Relations** — separate question; `MetaUnset` already exists there.
- **If-Match / optimistic concurrency** — dataentry-local; CalDAV ETag plumbing
belongs with CalDAV. `PatchEntity` is documented last-write-wins **per
property**, already better than today's per-entity.
- **Read-side ACL semantics** — unchanged.
- **`cascadeHost.WriteEntity` manager bypass** — BUG-KIMZRK.
- **`restoreOntoLive`**, **`syncHandler.putEntity`** — deliberate whole-entity
replaces; correct for restore / sync contract.
- **dataentry PATCH migration** — reference semantics, already correct.
- **Adding clear-content to MCP** — RR-S3U18Q: preserve current behaviour.

**Acceptance Criteria:** the 8 on TKT-80EWGM, plus three added by review: AC9
locked-entity patch is refused; AC10 elevated (`bypass_acl`) patch bypasses the
field gate **and** emits the bypass audit record; AC11 automation may set a
property the caller could not author.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — internal consolidation of four existing in-tree
implementations, not a survey of unfamiliar approaches. Every open question was
answered by in-tree precedent, which is what a research doc would have gone
looking for.

**Existing Solutions (all in-tree prior art):**

- **`createCore`** (`core.go:88`) — *the* structural precedent. Free function
over `Deps`, deliberately not a method, containing **no ACL and no
attribution**. `updateCore` mirrors it exactly.
- **`buildCandidateEntity`** (`core.go:~130`) — precedent for splitting "compute
candidate" from "persist", so dry-run cannot drift from the real path.
- **`DeleteEntity`** (`manager.go:663-673`) — the read-then-authorize shape
`PatchEntity` must adopt, with the tradeoff already documented.
- **dataentry PATCH** (`write_handler.go:406-430`) — reference merge semantics
incl. the unset-after-upsert ordering rule (`:411-415`); and its gate ordering
(`:322` then `:376`) is the model for §3.
- **`entity.RelationOptions`** (`writeapi.go:99-111`) — naming precedent:
`MetaUnset []string`, `Content *string`, with the tri-state rationale.
- **`ApplyEntity`** (`apply.go:85-91`) — the `IsLocked` guard precedent.
- **`lua.WriteDeps.EntityManager`** (`lua/deps.go:94`) — consumer-side interface
pattern; no arch-lint edge.
- **`internal/affordances`** — already exposes
`PolicyResolver.FieldVerdicts(ctx, *entity.Entity)` and does not import
dataentry. Home for shared gate logic.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### 1. Extract `updateCore` (standalone commit, no behaviour change)

Boundary is explicit (RR-U01ILQ), mirroring `createCore`:

| Stays in entry points | Moves into `updateCore` |
|---|---|
| `withStoreAttribution` (`manager.go:556`) | validate(pre) + partition (`:566-571`) |
| nil / id guards | automation + re-validate (`:580-604`) |
| `authorizeAndAudit` (`:559`) | transitions (`:606-616`) |
| raw `GetEntity` | unique (`:618-623`) |
| `IsLocked` guard | store write (`:625-630`) |
| `FieldGate` | audit (`:632-635`) |
| | cascade (`:641-655`) |

```go
func updateCore(ctx context.Context, deps Deps, e, oldEntity *entity.Entity, ...) (*entity.UpdateResult, error)
```

`oldEntity` is **passed in**, not re-read (RR-GM92KZ) — both entry points
already hold it, so the extraction *reduces* reads from two to one on the patch
path. Keeping authorize out prevents `PatchEntity` double-authorizing (which
would emit two denied-write audit rows on a deny) and stops `updateCore`
becoming a second public write surface.

Load-bearing ordering comments (audit-before-cascade at `:632`,
re-validate-after-automation, exclude-self on unique) carry over **verbatim**.

### 2. `PatchEntity`

```go
// internal/entity/writeapi.go — beside RelationOptions
type EntityPatch struct {
    Properties map[string]any // upserts
    Unset      []string       // removals, applied AFTER upserts
    Content    *string        // nil = leave body untouched
}

// internal/entitymanager
func (m *Manager) PatchEntity(ctx context.Context, id string, p EntityPatch) (*entity.UpdateResult, error)
```

`writeapi.go` is the verified-correct home: its own doc says these types live in
`entity` "because they're consumed by code that has no reason to import the
entitymanager package — autocascade scripts, dataentry handlers, mcp tools, lua
bindings, cli helpers." No cycle: `entity` imports only `metamodel`.

### 3. Ordering (REVISED — RR-32XA5V, RR-0V0TVB, RR-0QWLRC)

```
withStoreAttribution(ctx)
  → raw GetEntity(id)                     // ungated: write-prep, the whole point.
                                          //   Also needed to learn Type for the ACL subject.
  → IsLocked() guard                      // apply.go:85 precedent
  → authorizeAndAudit(OpUpdate, {stored.Type, id})
  → FieldGate.CheckFieldWrite(...)        // AFTER authorize
  → Clone() → apply Properties → delete Unset → set Content if non-nil
  → updateCore(ctx, deps, e, oldEntity)
```

**Why the gate moved after authorize.** The original plan had it before, which
is an oracle. dataentry gates first deliberately (`write_handler.go:318-324`:
*"A 400 / 412 / 422 here would be an existence oracle"*, RR-FGUZ) and runs
`validateFieldWrite` only at `:376`. Two things leak under the wrong order —
**existence** (not-found vs denied) and **content** (field verdicts are
value-dependent, so allow-vs-deny on a fixed patch reveals stored values). Both
are "data", which is secret. Note the *policy* being named in the denial is
**not** a leak — config is public per root CLAUDE.md, so
`AffordanceDenialError.Attribution` may keep naming the rule.

**Why authorize can't move earlier.** `acl.EntitySubject` needs `Type`, and
`PatchEntity` receives only an ID — so it must read first. This is exactly
`DeleteEntity`'s documented shape. Rejected: taking `type` as a parameter, which
would let a caller assert a type disagreeing with the stored row (the class
`ApplyEntity` guards with `ErrTypeImmutable`).

### 4. Field-write gate

Narrow consumer-side interface declared **in `entitymanager`**
(`TemplateLoader`/`TransitionEnforcer` pattern) — so **no `.go-arch-lint.yml`
change is needed**:

```go
// The gate constrains CALLER-AUTHORED changes only. Automation-derived
// properties (automation.Result.PropertiesSet) are system writes and are
// deliberately NOT gated: the gate enforces affordance parity with what the
// resolver would surface on GET for this principal, and automation is the
// system acting, not the principal. (RR-00ERM9)
type FieldWriteGate interface {
    CheckFieldWrite(ctx context.Context, e *entity.Entity, set map[string]any, unset []string) error
}
```

**Required** in `Deps`, rejected-if-nil in `New()`, with an exported
`entitymanager.AllowAllFieldGate{}` mirroring `acl.NopACL{}`. Cost is ~2
production sites and ~48 test sites — realistic, and matches the house pattern
(`Audit`/`ACL` are already required with named no-ops). A silently-nil gate is
the "forgotten wiring must not become an ACL bypass" failure RR-X9NVHI names.

**Elevation is total** (RR-BA1NIV): `bypassACL` bypasses the field gate too. An
operator writing `rela.bypass_acl(...)` can already read `acl.yaml` in full —
config is not secret — so a gate blocking them conceals nothing and is merely a
silent obstacle, i.e. the "half-elevated contract" `lua/deps.go:126-133`
rejects. `recordACLBypass` still fires: audit is attribution, not secrecy.

### 5. Migrations

- **lua** (`runtime.go:1724`) → `PatchEntity`; deletes `ReadDeps.WritePrepStore`.
- **mcp** (`tools_entity.go:218`) → in-band `nil` becomes `Unset`. Empty content
maps to `Content: nil` (**preserve** today's inability to clear the body,
RR-S3U18Q). Also removes a latent un-cloned-mutation hazard (`:200` mutates the
store-returned entity in place; safe today only by backend accident).
- **cli** (`update.go:38`) → gains clear-property and clear-content; wires
`AllowAllFieldGate{}` (operator trust boundary, full access by design).

**Alternatives rejected:** delegate-to-`UpdateEntity` (no seam for the gate,
double read); reimplement `UpdateEntity` via `PatchEntity` (breaks its
documented in-place-mutation contract, and `ApplyEntity` needs whole-entity
replace); add `PatchEntity` to the `EntityManager` interface (documented
"transitional… slated for removal"; breaks every fake); import `affordances`
directly (needs an arch-lint change the interface approach avoids); partial-body
edits (whole-body `*string` matches every precedent).

**Files to modify:** `internal/entity/writeapi.go`;
`internal/entitymanager/{core,manager}.go` + `entitymanagertest/`;
`internal/affordances/`; `internal/dataentry/affordances.go`;
`internal/lua/{deps,runtime}.go`; `internal/mcp/tools_entity.go`;
`internal/cli/update.go`; wiring in `internal/appbuild/*`,
`internal/dataentry/app.go`, `internal/docs/runtime.go`, `internal/script/*`;
root `CLAUDE.md` + `internal/entitymanager/CLAUDE.md`; `docs/cli-reference.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

| Input | Validation | On invalid |
|---|---|---|
| `id` | store ID validation; miss → `ErrEntityNotFound` | uniform not-found |
| `Properties` keys | `declaredProperties` **allowlist**; unknown → `RuleFieldHidden` | preserves the F8 side-channel closure |
| `Properties` values | `Meta.ValidateEntity`, DEC-HWZHA hard/soft split | hard aborts; soft warns |
| `Unset` keys | same allowlist + gate | denial |
| `Content` | unchanged | — |

1. **Raw write-prep read** — deliberate; consolidating four raw reads into one
auditable location is the improvement.
2. **Ordering is the security-critical part** — see §3. Existence and
value-dependent content must not leak before authorize. Config/policy naming is
explicitly fine (config is not secret).
3. **`IsLocked` guard** — without it, patching a git-crypt-locked entity writes
the cleartext shell over encrypted content: the ticket's own erasure class, via
encryption instead of redaction.
4. **Audit** — `PatchEntity` is NOT in the `{Create,Update,Delete,Rename}` set
that `internal/entitymanager/CLAUDE.md` says inherits audit mechanically, so
audit must come from the shared `updateCore`. Verify, don't assume.
5. **Elevation** — total, and audited via `recordACLBypass`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

| AC | Scenario |
|---|---|
| 1 | Integration w/ real `Deps`: automation fired, unique enforced, transition enforced, audit emitted, cascade ran. |
| 2 | **Inverse test** — redacted-read caller patches ONE visible prop; every hidden prop byte-identical after. |
| 3 | Table {visible,hidden} × {set,unset,absent}. |
| 4 | Body tri-state: `nil` / `""` / `"x"`. |
| 5 | `WritePrepStore` gone; `luaUpdateEntity` holds no `store.Store`. |
| 6 | MCP nil-delete + MCP **cannot-clear-body** + CLI regressions all green. |
| 7 | CLI patches a hidden property successfully (`AllowAllFieldGate`). |
| 8 | `just arch-lint lint test coverage-check`. |
| **9** | Patching a locked entity is refused; stored bytes byte-identical. |
| **10** | Elevated patch of a gated field succeeds AND emits the bypass audit record. |
| **11** | Principal who cannot author `status` triggers an automation that sets `status` → succeeds. |

**Ordering tests (from RR-32XA5V) — the security regression net:** a principal
with **no write authority** patches (a) a nonexistent id and (b) an existing
entity whose field policy would deny — assert **both** return the same
authorization failure, and that neither returns `AffordanceDenialError`. This is
what pins the gate behind authorize.

**Integration approach:** AC1/AC2 run end-to-end **through the cascade path**
(autocascade → LuaScriptRunner → patch), preserving the coverage RR-J4518A
demanded — that finding exists precisely because the original test only used a
directly-constructed runtime.

**Edge Cases:** empty patch (no-op; document whether it bumps `UpdatedAt`); key
in both `Properties` and `Unset` → cleared; unset absent key → no-op; unset
undeclared key → `unknown_property_unset_key` warning; nil `Properties` +
non-empty `Unset`; entity not found → `ErrEntityNotFound`; `Clone()` always
allocates a non-nil map so nil-Properties entities patch fine; non-string values
assigned directly (not via `SetString`) to preserve `any` typing; unsetting a
**required** property succeeds with a warning (DEC-HWZHA, verified:
`metamodel/validation.go:92-100` → `entitymanager/validation.go:22-27`, code
`required_property_unset`); concurrent patches are last-write-wins per property.

**Negative:** hidden set → denied; hidden unset → denied (parity); read-only
same-value → still denied (strictness deliberate); undeclared →
`RuleFieldHidden` not "unknown field"; illegal transition → transition error;
unique violation → unique error; `New()` with nil `FieldGate` → constructor
error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

| Risk | Mitigation |
|---|---|
| `updateCore` extraction regresses the hottest write path | Standalone behaviour-preserving commit, suite green before `PatchEntity` exists; verbatim ordering comments. |
| Required `Deps.FieldGate` breaks ~50 construction sites | Compiler finds all; `AllowAllFieldGate{}` fixture first. ~2 production + ~48 test, measured. |
| Gate extraction changes dataentry denial behaviour (rule names are a wire contract) | dataentry delegates rather than being rewritten; existing affordance tests are the net. |
| `WritePrepStore` deletion touches many wiring sites | Compiler-guided; **replace** the two identity guard tests with new-invariant equivalents, don't delete. |
| Getting the gate/authorize order wrong again | Explicit ordering tests above; the order is now documented with its rationale in `PatchEntity`'s godoc. |
| Scope creep toward BUG-KIMZRK | Out of scope; changes cascade failure semantics, needs its own design. |

**Effort:** l — confirmed, slightly up after review (IsLocked guard, 3 extra
ACs, ordering tests).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

- [x] root `CLAUDE.md` — **the point of the ticket**: replace "Never redact a read
that feeds a write" with "use `PatchEntity`". The prose guards a footgun this
removes.
- [x] `internal/entitymanager/CLAUDE.md` — document `PatchEntity`; note it is NOT
in the auto-audit `{Create,Update,Delete,Rename}` set.
- [x] `docs/cli-reference.md` — CLI gains clear-property/clear-content.
- [x] Lua API docs — decide whether to expose unset on `rela.update_entity`.
- [x] ~~`docs/metamodel.md`, `docs/data-entry.md`~~ (N/A: no metamodel change;
dataentry behaviour is unchanged — it is the reference semantics, not a
migration target)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 7, all `addressed`.

| ID | Sev | Outcome |
|---|---|---|
| RR-32XA5V | critical | Gate moved after authorize. Rationale narrowed: existence + value-dependent content leak (real); policy-naming leak (withdrawn — config is public). |
| RR-0V0TVB | significant | Adopt `DeleteEntity` read-then-authorize shape; documented, not accidental. |
| RR-BA1NIV | significant | Elevation is **total**; `recordACLBypass` still fires. |
| RR-00ERM9 | significant | Automation ungated — status quo, now written into the godoc + pinned (AC11). |
| RR-0QWLRC | significant | `IsLocked` guard moved INTO scope (AC9). |
| RR-GM92KZ | minor | `oldEntity` passed into `updateCore`; no `store.Tx`. |
| RR-U01ILQ | minor | Extraction boundary specified; standalone first commit. |
| RR-S3U18Q | minor | MCP: preserve cannot-clear-body; un-cloned mutation fixed by construction. |
