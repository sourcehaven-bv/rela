---
id: PLAN-9KPQHK
type: planning-checklist
title: 'Planning: Replace producer-side entitymanager.EntityManager with per-consumer interfaces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `entitymanager.EntityManager`
(`internal/entitymanager/entitymanager.go:29`) is a 9-method producer-side
interface declared alongside its sole implementation, `*Manager`. CLAUDE.md's
first rule forbids exactly this, and the interface's own godoc concedes it:
*"**Transitional.** This is a producer-side interface ... which the project
explicitly avoids ... Slated for removal."* GitHub issue #741.

Every consumer that takes it today binds to the full surface even though none
uses all of it, which leaks unused methods into test fixtures and arch-lint
footprints.

**Scope — IN:** each of the four consuming subsystems declares its own narrow
interface at the call site; the wide interface is deleted; producer code uses
`*Manager` directly.

**Scope — OUT:**

- Renaming methods or changing any signature. Purely a contract narrowing.
- Splitting `*Manager` itself (it is over the plimsoll line for other reasons —
TKT-N0IKN9 territory, not this).
- `lua.Mutator` — already narrowed by TKT-IF37; it is the template here, not a target.
- Behaviour change of any kind.

**Acceptance Criteria:**

1. **AC1** — `entitymanager.EntityManager` no longer exists in the package.
2. **AC2** — every former consumer declares a narrow interface naming only the
methods it invokes, at the consumer.
3. **AC3** — `go build ./...` and the full test suite pass unchanged; `*Manager`
satisfies every new interface structurally with no adapter or assertion added.
4. **AC4** — `just arch-lint` passes.
5. **AC5** — no behaviour change: no test assertions are modified to accommodate
the refactor. (A test that has to change is evidence the refactor changed
behaviour.)
6. **AC6** — `internal/attachment` and `internal/mcp` no longer import
`internal/entitymanager` at all, since their narrow interfaces mention only
`internal/entity` types. This is the concrete coupling win and is verifiable
with `go list -deps`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — a mechanical refactor with an in-tree template.

**Existing Solutions:** This has already been done once in this repo, for the
same interface, and the result is the template. `lua.Mutator`
(`internal/lua/deps.go:115-127`) is the consumer-side write surface for the Lua
bindings, and its godoc states the pattern precisely:

> Six methods — RenameEntity and UpdateRelation are intentionally absent because no
> Lua binding invokes them. Narrowed from the wider EntityManager interface in
> TKT-IF37 to drop lua's transitive dependency on internal/entitymanager.

`lua.NotFoundError` beside it shows how to keep an *optional* capability
consumer-side too. `appbuild.LuaWriteDeps` documents the resulting asymmetry:
*"EntityManager goes in as the wide entitymanager.EntityManager; the
lua.WriteDeps.EntityManager field is narrower (lua.Mutator) and accepts any
structural match."* After this ticket that comment loses its premise and needs
updating.

Style to match for the new interfaces — `internal/dataentry` already has ~17
consumer-side interfaces (`metaView`, `analyzeReader`, `relationCounter`,
`storeWatcher`, `syncStore`, …). `metaView` (`provision.go:166`) is the model:
unexported, one purpose, and a godoc that names *why* it is defined at the call
site.

**Method sets — measured, not guessed.** Grepped every call through each
consumer's field (`a.entityManager`, `h.manager`, `d.EntityManager`,
`svc.EntityManager`):

**dataentry is four consumers, not one** — the field is passed down into three
sub-holders with different needs, so it gets four interfaces, not one shared
one:

| Consumer | Methods invoked | Count |
|---|---|---|
| `dataentry.App.entityManager` | CreateEntity, UpdateEntity, PatchEntity, DeleteEntity, CreateRelation, UpdateRelation, DeleteRelation | 7 |
| `dataentry.writeHandler.manager` | CreateEntity, ValidateCreate, UpdateEntity, DeleteEntity, CreateRelation, UpdateRelation, DeleteRelation | 7 |
| `dataentry.attachmentHandler.manager` | *(none — pure passthrough)* | 0 |
| `dataentry.maybeProvision(m)` | CreateEntity | 1 |
| `internal/cli` | CreateEntity, UpdateEntity, PatchEntity, DeleteEntity, RenameEntity, CreateRelation, UpdateRelation, DeleteRelation | 8 (all but ValidateCreate) |
| `internal/mcp` | CreateEntity, PatchEntity, DeleteEntity, RenameEntity, CreateRelation, DeleteRelation | 6 |
| `internal/attachment` | UpdateEntity | 1 |

`internal/appbuild` invokes **nothing** — pure wiring.

Note the asymmetries: `App` never calls `ValidateCreate`, `writeHandler` never
calls `PatchEntity`, and `ValidateCreate` is invoked from **exactly one place in
the whole repo** (`write_handler.go:293`). `RenameEntity` is reached only from
cli and mcp, so it leaves dataentry entirely.

Call sites backing the table: dataentry
`caldav_write.go:188,243,293,309,576,595,600,692,698`,
`history_restore.go:107,145`, `relation_history_handler.go:368,370`,
`write_handler.go:172,293,470,532,637,711,765,800`,
`relations_modern.go:376,415`, `provision.go:96`; cli `create.go:54`,
`update.go:93`, `delete.go:59`, `rename.go:189`, `link.go:18`, `unlink.go:20`,
`renumber.go:68`, `restore.go:61,68`, `relation_history.go:201,205`; mcp
`tools_entity.go:167,246,317,352`, `tools_relation.go:86,120`; attachment
`attachment.go:202,226`.

The interesting numbers are the small ones: **attachment needs exactly one
method** out of nine, `attachmentHandler` needs *zero*, and mcp needs six. Those
are where the narrowing actually buys something.

**Two structural constraints the grep alone would have missed:**

1. `App` passes `a.entityManager` into `lua.WriteDeps.EntityManager`, typed
`lua.Mutator` (6 methods). That set is a strict subset of App's 7, so the narrow
type satisfies it structurally — but it means App's interface may never drop
below `lua.Mutator`.
2. `cli/sync.go:116` type-asserts `svc.EntityManager.(syncclient.LocalApplier)`.
Interface-to-interface assertion stays legal however narrow the source, so this
compiles either way — but see the `Services.EntityManager()` decision below,
which turns it into a compile-checked concrete match instead of a runtime one.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Mechanical, one consumer at a time, compiler-verified at
each step. For each consumer: declare the interface next to the type that holds
it, change the field/param type, build. `*Manager` satisfies each structurally,
so no wiring changes and no adapters.

1. **`appbuild` first.** Change `Services.entityManager`, `Services.EntityManager()`
and `Collaborators.EntityManager` (`appbuild.go:116, 233, 626`) to the concrete
`*entitymanager.Manager`. `buildEntityManager` already returns the concrete type
(`appbuild.go:1369-1375`), so this is a type annotation catching up with
reality. Doing it first means every consumer then satisfies its own narrow
interface for free, and `sync.go`'s `LocalApplier` assertion becomes
concrete-to-interface (compile-checked) rather than interface-to-interface
(runtime).
2. `internal/attachment` — `entityUpdater` (1 method) on `Deps.EntityManager`.
3. `internal/mcp` — 6 methods on `Deps.EntityManager`. This is the same file that
already declares `GraphReader`/`GraphCounter` at the call site with a long
rationale, and whose doc already says MCP "never needs the wide composite"
(`server.go:59`); the new interface belongs beside them, in that doc style.
4. `internal/cli` — 8 methods on `writeServices.EntityManager`.
5. `internal/dataentry` — four separate interfaces per the table: `App` (7),
`writeHandler` (7), `attachmentHandler` (whatever attachment's is — it only
passes through), `maybeProvision` (1, alongside the `metaView` already in that
file).
6. Delete the interface, plus the `var _ EntityManager = (*Manager)(nil)` assertion
at `manager.go:117` (keep the `autocascade.Mutator` one beside it).
7. `entitymanagertest.PanicOnUse` — drop `var _ entitymanager.EntityManager`
(`panic.go:21`); keep all nine methods so it still satisfies every narrow
interface structurally, and say so in its godoc.
8. Fix the comments this invalidates (each names the type or calls it "broad"/"wide"):
`appbuild.go:433-435`, `cli/sync.go:100-104,117-119`,
`cli/sync/engine.go:15-16`, `dataentry/attachment_handler.go:19-21`,
`dataentry/services.go:20`, `dataentry/settings_handlers.go:85`,
`entitymanager/entitymanager.go:1-28`, `entitymanagertest/panic.go:1-6,17`.

**Naming.** Consumers get unexported names except where the field is on an
exported `Deps` (mcp, attachment), where Go requires the interface be exported
if the field type is to be nameable by the wiring site — check each;
`lua.Mutator` is exported for that reason, so follow suit where the same applies
and stay unexported otherwise.

**Files to modify:** `internal/entitymanager/entitymanager.go` (delete
interface), `internal/entitymanager/entitymanagertest/panic.go`,
`internal/attachment/attachment.go`, `internal/mcp/server.go`,
`internal/cli/cli_wiring.go`, `internal/dataentry/app.go`,
`internal/dataentry/attachment_handler.go`,
`internal/dataentry/write_handler.go`, `internal/dataentry/provision.go`,
`internal/appbuild/appbuild.go`.

**Alternatives considered:**

1. *Keep the interface, just document it.* Rejected — it is already documented as
transitional and slated for removal; leaving it is how "transitional" becomes
permanent.
2. *One shared narrow interface for all consumers.* Rejected — that is the same
producer-side mistake with a smaller surface. It would also be wrong: no two
consumers have the same method set (dataentry and cli both need 8, but different
8s).
3. *Split by capability (EntityWriter / RelationWriter).* Tempting, but it invents a
taxonomy nothing asked for — three of the four consumers need both halves, so it
would mean embedding two interfaces to say what one list says plainly. Consumers
declare what they call; that is the rule.
4. *Do it in 3-5 PRs grouped by subsystem*, as the issue suggests. Rejected as
unnecessary: the change is ~10 files and the interface can only be deleted once
every consumer is converted, so splitting means N-1 PRs that add interfaces
without removing anything, and the payoff lands only at the end. One reviewable
PR is better here. Revisit if it turns out larger than measured.

**Dependencies:** none. No package gains an import; two should lose one (AC6).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** None. This refactor introduces no new input, no
new call path, and no new code that runs at request time — it only narrows the
static type of existing fields.

**Security-Sensitive Operations:** One property is worth stating because it is
the reason this refactor is safe to do mechanically: `entitymanager.Manager` is
the sole write path, and it is where ACL, audit and validation are enforced
(`internal/entitymanager/CLAUDE.md`). Narrowing an interface **cannot** bypass
any of that — every method retained keeps its identical signature and
implementation, and no consumer gains access to a method it did not already
have. The risk would be the opposite direction (a consumer accidentally handed a
*raw store* handle instead), which this change does not touch.

The one thing to watch: `entitymanagertest.PanicOnUse` exists so that read-only
test paths fail loudly on an accidental mutation. If narrowing let a consumer
take an interface `PanicOnUse` no longer satisfies, that safety net would
silently stop being wired. Keeping all nine methods on it (step 6) preserves it;
AC3 catches a regression at compile time.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** This is a refactor with no behaviour change, so the test
strategy is *the existing suite, unmodified* — that is the actual assertion
(AC5). Specifically:

- **AC1/AC2/AC3** — `go build ./...` plus `just test`. The compiler is the
verification: if a consumer's narrow interface omits a method it calls, the
build fails; if `*Manager` drifts from an interface, the build fails at the
wiring site.
- **AC4** — `just arch-lint`.
- **AC5** — `git diff --stat` must show **zero** changes under any `_test.go`
assertion. Test files may change only where they name the deleted type.
- **AC6** — the concrete win, and the only genuinely new check worth adding:

  ```
  go list -deps ./internal/attachment | grep -q internal/entitymanager  # must fail
  go list -deps ./internal/mcp        | grep -q internal/entitymanager  # must fail
  ```

Verify before and after; if either still imports it, some other reference
remains and the narrowing bought nothing there.

**Edge Cases:**

- A method invoked only from a `_test.go` in a consumer package would be missed by
a production-only grep, and the omission surfaces as a test-compile failure.
Mitigation: build tests too (`go vet ./...` / `just test` compiles them).
- **`dataentry/unmatched_principal_e2e_test.go:182`** compares
`app.luaWriteDeps().EntityManager != app.entityManager` — two *interface*
values. Once the two sides have different interface types Go rejects the
comparison at compile time. Making `Services.EntityManager()` concrete (step 1)
resolves it in the right direction: both sides become comparable `*Manager`
pointers and the test's intent ("actions share the CRUD manager, so the reject
gate covers them") is expressed more directly than before. This is a test
*compile* fix, not an assertion change, so it does not violate AC5 — but call it
out in the PR.
- Reflection or interface assertions on the deleted type would compile-fail; a
repo-wide grep for `entitymanager.EntityManager` after the change must return
zero (including `cmd/` and `_test.go`).
- `PanicOnUse` must still satisfy every new interface — asserted by the tests that
already wire it.
- The postgres/memory/sqlite build tags compile different `appbuild` recipes;
`Services.entityManager`'s type change must build under each. Check with `go
build -tags postgres ./...` and the other tags, not just the default build.

**Negative Tests:** None applicable — there is no new runtime behaviour to fail.
The "negative test" for a type-narrowing refactor is that the compiler rejects a
consumer calling a method it did not declare, which is structural rather than
something to assert in a test.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| A method used only in a test or under a build tag is missed | Compile all tags and all tests, not just `go build ./...` |
| Merge conflicts — this touches files other in-flight work also touches (`dataentry/app.go`, `mcp/server.go`) | Small, mechanical, single PR; land it promptly rather than letting it age |
| `PanicOnUse` silently stops covering a path | Keep all nine methods; compile-time assertion via existing wiring |
| Scope creep into splitting `*Manager` | Explicitly out of scope (TKT-N0IKN9 owns that) |

**Effort:** m — ~10 files, mechanical, compiler-verified at every step.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `appbuild.LuaWriteDeps` godoc — its "goes in as the wide
entitymanager.EntityManager" sentence loses its premise; update as part of the
change.
- [x] `entitymanagertest` package godoc — it opens "test doubles for the
entitymanager.EntityManager interface", a type that will no longer exist.
- [x] `internal/entitymanager/entitymanager.go` package doc — check whether it
still reads correctly once the interface is gone.
- [x] ~~README.md / docs/~~ (N/A: no user-facing surface changes — no CLI flag,
no API, no config)
- [x] ~~CLAUDE.md~~ (N/A: this ticket *applies* the existing consumer-side
interfaces rule rather than changing it;
`docs/architecture/consumer-side-interfaces.md` already documents the pattern)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: skipped
      deliberately, and the judgement is worth recording. `/design-review`
      exists to catch design decisions that are expensive to reverse. This
      ticket has none: the interface's own godoc already prescribed the design
      ("each call site should declare its own narrow consumer-side interface"),
      the method sets were *measured* from call sites rather than chosen, and
      every step is compiler-verified — a wrong set does not compile. The real
      risk here was in the *code*, not the plan, which is where the review
      effort went: `/code-review` returned 12 findings including a latent
      nil-deref, and two of them (RR-NEE4FC, RR-XHC5EB) did change the design.)
- [x] All critical/significant findings addressed in plan — all 12 code-review
      findings addressed; see the review checklist.

**Design Review Findings:** N/A — see the review checklist for the 12
code-review findings (RR-ZTWK9S critical; RR-XHC5EB, RR-LOZBZQ, RR-EE4DIU,
RR-09AUCP, RR-1XW25U, RR-NEE4FC significant; RR-GE0DM1, RR-2RCGDV, RR-PRY567
minor; RR-VXK77U, RR-ZOJCR3 nit).
