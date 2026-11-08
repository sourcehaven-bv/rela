---
id: PLAN-G8YMBDB
type: planning-checklist
title: 'Planning: rela test v1: .test.lua runner with fixtures, principals, fake clock and mail capture'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- A `rela test [paths...] [--run <pattern>] [--seed N]` command.
  - It finds `*.test.lua` under `tests/` (or the given paths) and runs every `t.case`.
  - It is **not** in `requiresProject`. It finds the root with `project.Discover` and never opens the live store, the bleve lock, the sqlite lock or a database connection.
- A fresh world for each case, built by a new untagged `appbuild.NewInMemory` recipe:
  - memstore plus `search.LinearSearch` (no bleve, so the postgres no-bleve gate holds);
  - `Config.FS = storage.NewMemFS()`, so every write (state KV, migration record, comments, piles, leases, lock files) stays in memory;
  - `projectConfig = rootfs.New(root)`, so config is read from disk, the same seam the sqlite recipe uses;
  - a full `prepare` + `assemble` per case with that case's options; `Services.Close` at the end of the case.
- Fixtures load with `fsimport` Copy into the case's memstore, as given state: no ACL, no automations, no validation, tool `rela-test`.
- A test Lua VM with the scripts' safe library set, plus a `rela.test` module:
  - `t.case`, `t.fixtures`, `t.as(user)`, `t.principal{roles}`, `t.clock`, `t.now`, `t.advance` (clock only), durations (`t.days(3)`);
  - `t.random` (seeded);
  - generated data by default: `create(type, props?)` fills required properties from the schema; `t.fake` (schema-aware `value(type, prop)`, `entity(type, overrides)`, plus `name`, `email`, `sentence`, `date{within}`, `pick(list)`);
  - writes: create, update, link, unlink, transition, delete;
  - reads: get, list (filter), through the principal's visible reader;
  - assertions: `t.expect(v).to_be/to_equal/to_contain/to_be_nil`, `t.expect_error`, `t.expect_denied`, `t.expect_mail{to,subject_contains}`, `t.mail()`, `t.expect_access(principal, table)` (the `acl map --expect` shape, moved to `internal/aclmap`).
- Principals and roles:
  - `t.as(user)` names a principal. Its roles come from the operator's real policy (`assignments`, the member-of walk over fixture entities, the everyone role).
  - `t.principal{roles = {...}}` creates a user entity in the case's store and a **per-case clone** of the policy with an `assignments` entry for it, passed with `WithACL`. The shared policy is never mutated.
  - IdP claim roles are not used. They only select `asserted_role_assignments`, so they would grant nothing.
- Determinism seams in appbuild:
  - **`WithClock(func() time.Time)`.** Threaded into:
    - automation `TemplateVars`;
    - every predicate evaluator: automation, validation, writerule, queryscopes, scopes;
    - computed properties;
    - Lua runtimes built for automations, actions and scripts;
    - copy/convert interpolation;
    - memstore `UpdatedAt` and soft-delete stamps;
    - the lease clock.

Audit and version timestamps stay on the wall clock (documented).
- **Mail:** `mail.yaml` is still parsed, so `RecipientPolicy` and the base URL apply. Only the transport is replaced, with one shared `MemorySender` per case. In the test world, outbox delivery is flushed synchronously before `t.mail()` returns.
- **HTTP and AI:** a per-runtime `lua.WithHTTPClient` option with a refusing transport, and a refusing AI provider that takes precedence over `ai.yaml`.
- **Output files:** `rela.write_file` writes to a temporary output directory outside the project root, removed after the case.
- **Secrets:** the host config contains no secrets or tokens.
- **Jobs:** no new queue. `background: true` automation actions already run in the foreground unless `WithBackgroundAutomationJobs` is set, and the test world does not set it.
- Output: one line per case (ok / not ok with location and message), a summary with the seed, and exit code 1 on any failure. `-o json` for CI.

Out of scope (backlog tickets under FEAT-IODR9ST):
- scheduler ticks (TKT-NA2FG3A);
- HTTP/AI stubs and recording (TKT-HB313W5);
- trace, coverage and impact (TKT-4GYJ8ER);
- self-explaining failures and docs (TKT-KPFCURD);
- mutation testing (TKT-7T5R9HV);
- property-based tests (TKT-DHEA8T7);
- record mode (TKT-8BE74AM);
- version differential (TKT-2O50NB0);
- nudging hints (TKT-SFCONIN);
- operator-chosen backend and reading config from `rela.db` (TKT-DAUS290);
- reproducible entity IDs and automation-side randomness.

**Acceptance Criteria:**
1. A case that updates an entity so an automation fires sees the automation's effect (property set, relation or entity created, `background: true` action applied) in the same case.
2. Access, in one case:
   - a principal whose role holds the grant does a single write, and it succeeds (positive control);
   - a principal without the grant does the same write, `expect_denied` passes, and the write is not applied.
3. `t.clock("2026-03-01T09:00:00Z")` makes all of these see that instant: `{{today}}` in an automation, `today()` in a predicate (validation, writerule, scope), a computed property, `rela.today()` in automation Lua, and memstore `mod_time`.
4. Mail from an automation or a Lua action appears in `t.mail()` right after the write. Nothing goes over the network, even when `.rela/mail.yaml` configures SMTP. A recipient blocked by `RecipientPolicy` is still blocked.
5. An automation that declares the HTTP and AI capabilities and calls `http.get` or `ai.chat` fails the case with "real HTTP/AI calls are not allowed in rela test".
6. Nothing under the project root changes during a run. This is checked by hashing the tree before and after, on a project with comments, piles, a WriteFile script and data migrations.
7. Cases are isolated: a create in case A is not visible in case B.
8. A failing assertion reports the file:line of the `t.expect` call, and the command exits 1. When everything passes it exits 0.
9. The test VM has no `os`, `io`, `debug` or `load*`, and `math.random` is replaced by the seeded generator.
10. `tickets/` gets a small `tests/` suite that passes in CI, with no literal timestamps. It covers checklist auto-creation on a status change, a validation rule, and an access expectation.
11. `create(type)` with no properties succeeds with **zero validation warnings** on a type that has one required property of each kind: `string`, `date`, `datetime`, `integer`, `decimal`, `boolean`, `enum`, `rrule` and `file` (a small generated attachment). A type with a `manual` ID gets a generated id.
12. A run prints its seed. Rerunning with `--seed N` reproduces every `t.fake` / `t.random` value and the clock. Entity IDs and randomness inside automations are not reproducible, and the guide says so. Adding a case does not change the generated values of the existing cases.
13. Without `t.clock(...)`, the clock starts at an instant derived from the seed, not the wall clock.
14. `rela-sqlite test` runs while another process holds the project's database lock.
15. On a project whose required config exists only in `rela.db`, `rela test` exits 1 with a message that names the missing files and points to TKT-DAUS290's feature.
16. After 100 cases, no goroutines are left behind (leak check).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: approach discussed with the user in-session; options, rationale and design review recorded in this checklist)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (see above)

**Existing Solutions:**
- Lua test libraries (busted, luaunit) assume a full Lua with `os`/`io`, so they do not fit the sandbox. A small `t` module of our own is less code than adapting one.
- In the codebase:
  - `appbuild_memory.go` (build tag `memorybackend`) already pairs memstore and LinearSearch with `prepare` / `assemble`. v1 moves it into an untagged `NewInMemory`, and the tagged `New` calls it.
  - `Config.projectConfig` (the sqlite recipe, FEAT-UP14BT) separates where config is read from and where the FS writes.
  - `fsimport` Copy (`fsimport/copyinto.go`) copies a filesystem project into an open store with the trust boundary "no ACL, no automations, no validation". It handles faces, relations, attachments, comments and the migration record.
  - `appbuild/appbuildtest` is a hand-copied wiring that drifts, so it is rejected. `internal/testutil` imports `testing`, so a production binary cannot link it.
  - These clock seams exist but are not connected to one source: `lua.WithClock`, `predicatefns.NewEvaluatorWithClock`, `computed.CompileWithClock`, `automation.TemplateVars.Now`, `lease` `Clock`.
  - `mail.SenderFor` / `NewLuaSender` (transport vs policy split), `mail.MemorySender` and `lua.WithAIProvider` exist.
  - `cli/acl_expect.go` `accessExpectation` gives the `t.expect_access` format; it moves to `internal/aclmap`.
- Fakers:
  - `brianvoe/gofakeit` (MIT, seedable) supplies names, emails and sentences, wrapped behind `t.fake`.
  - Schema-aware generation is ours, modelled on `testutil.EntityFor`, but seeded.
  - rela's own Go tests already prefer random fixtures; the operator API follows that house style.
- References: Rails/Django fixture-per-test; OPA `opa test` (policy tests next to the policy).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. **`appbuild.NewInMemory(root, opts...)`**, untagged, in `memrecipe.go`. It builds a `Config{FS: storage.NewMemFS(), Paths: <project context with Root=root>, projectConfig: rootfs.New(root), Audit: audit.NewMemory()}`, then runs `prepare`, opens memstore with LinearSearch, and calls `assemble`. `appbuild_memory.go` `New` delegates to it. I'll check that `prepare` and data-migration bootstrap only write through `Config.FS`; any writer that bypasses it gets routed through it or an option.
2. **New appbuild options** `WithClock`, `WithMailTransport`, `WithHTTPClient`, `WithAIProvider` and `WithOutputDir`, stored on the option set read by `prepare`/`assemble`. There are no new `Services` fields. Each case passes its own options to a full `NewInMemory`, so there is no shared mutable clock.
3. **Clock threading** through every site listed under Scope.
   - A guard test in `internal/archguard` scans the write-path packages (`automation`, `entitymanager`, `validation`, `writerule`, `scopes`, `computed`, `lua`, `store/memstore`, `lease`) for `time.Now()`.
   - It has an allowlist with a reason per entry; audit and version stamps are allowed.
4. **`internal/apptest`:**
   - `Runner`: discover files, run cases sequentially, collect results.
   - `World`: `NewInMemory` services, `fsimport` fixture load, memory mail, temporary output directory, Close.
   - `module.go`: the Lua module.
   - `fake.go`: seeded, schema-aware generators.
   - `report.go`: output.

The per-case seed is a hash of the run seed and the file and case name.
5. **Test VM:** gopher-lua with the shared safe library list (exported from `internal/lua`). `require` resolves from `tests/` and `lib/` through the rooted loader.
6. **Identity:** `principal.With(ctx, principal.VerifiedFrom(user, principal.ToolRelaTest, principal.Claims{}))`, then `appbuild.IdentityBinder(svc)`. `ToolRelaTest` is a new constant, listed in the `VerifiedFrom` godoc exceptions. `t.as` models a CLI or scheduler principal, so `UnmatchedVerified` is never set.
7. **`internal/cli/test.go`:** the Kong `TestCmd`, excluded from `requiresProject`. It refuses with a clear message when required config is not on disk (config-in-`rela.db` projects).
8. **`tickets/tests/`** sample suite, plus a CI step.
9. **CI dependency gate:** add `go list -deps` checks that the postgres build still links no bleve now that `apptest` and memstore are in `cmd/rela`.

Alternatives rejected:
- **YAML test format:** one format only (decided with the user).
- **fsstore over a temporary copy of the project:** slower per case and touches disk. memstore was chosen with the user.
- **In-memory bleve:** breaks the postgres no-bleve gate. LinearSearch ranking differs from bleve and postgres FTS; documented.
- **A synchronous job queue:** breaks the `Enqueue` contract and deadlocks inside memstore `Tx`. Foreground delivery of background actions already gives tests synchronous effects.
- **Reusing one `SharedBase` across cases:** options are fixed at `prepare` time, so a per-case clock would need shared mutable state. A full `prepare` per case is simpler; I'll measure it.
- **IdP claim roles for `t.principal`:** they grant nothing unless `asserted_role_assignments` lists them, so tests would pass for the wrong reason.
- **A new fixture parser:** `fsimport` Copy already handles faces and the bare-ref rules.
- **Driving through the data-entry HTTP API:** `cmd/rela` does not link `dataentry`; that decision can come later.

**Files to modify:**
- New: `internal/appbuild/memrecipe.go`; `appbuild_memory.go` delegates to it
- `internal/appbuild/appbuild.go`, `mail.go`, `scheduled_mail.go`, `queryscopes.go`, `leases*.go` (options and clock)
- `internal/automation/engine.go`, `template.go`; `internal/validation/validation.go`; `internal/writerule/writerule.go`; `internal/scopes/scopes.go`
- `internal/entitymanager/manager.go`, `copy_apply.go`, `conversion_plan.go`, `cascadehost.go` (clock for computed, interpolate and cascade Lua)
- `internal/store/memstore/memstore.go`, `softdelete.go` (clock option)
- `internal/lua/runtime.go`, `http.go`, `context.go`, `output.go` (shared safe libs, `WithHTTPClient`, provider and transport precedence, output dir)
- `internal/mail/script.go` (transport replacement that keeps the policy)
- `internal/principal/principal.go` (`ToolRelaTest`)
- `internal/aclmap/expect.go` (moved from `cli/acl_expect.go`); `internal/cli/acl_expect.go` uses it
- New: `internal/apptest/{runner,world,module,assert,fake,report}.go` + tests + `testdata/`
- New: `internal/cli/test.go`; `internal/cli/kong.go`
- New: `internal/archguard/testclock_test.go`
- `.go-arch-lint.yml` (component `apptest`), `.github/workflows/ci.yml` (dependency gate and `rela test` on `tickets/`)
- `docs-project/` (guide "Testing your app with rela test", CLI reference), `tickets/tests/`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- **Test files and fixtures:** operator-authored, the same trust level as `scripts/`. They run in the same sandbox, and `require` is confined to `tests/` and `lib/` by the rooted loader.
- **Fixture directory argument:** resolved inside the project root through `rootfs`. A path outside the root is rejected.
- **Principal names and roles:** free strings. They exist only in the case's in-memory store and per-case policy clone.

**Security-Sensitive Operations:**
- **No disk writes by construction:** `Config.FS` is a MemFS and config is read through a read-only rooted loader. The output directory is a temporary directory outside the root. AC6 is the backstop.
- **No live locks or connections:** the command is outside `requiresProject`, so no sqlite lock, bleve lock or `RELA_DATABASE_URL` is used.
- **Secrets:** the host config contains no secrets or tokens, so a test cannot read production credentials.
- **Mail, HTTP and AI:** captured or refused. `RecipientPolicy` still applies, so tests also exercise it.
- **Identity:**
  - Synthetic principals use the new `ToolRelaTest` tool and exist only in the test world.
  - The per-case policy clone never touches the shared policy.
  - Binding goes through `appbuild.IdentityBinder`, so the identity-edge allowlist is unchanged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- **AC1–5, 7, 11–13:** Go integration tests in `internal/apptest`, run against a fixture project in `testdata/`. Each test runs a `.test.lua` file and checks the result report. AC11 runs across 200 seeds.
- **AC6:** hash the tree before and after a run, on a testdata project with comments, piles, a WriteFile automation and a pending data migration.
- **AC8:** a CLI test checks the exit codes and the location in the output.
- **AC9:** a test file calls `os.time`, `io.open` and `load` and expects errors.
- **AC10:** `rela test --project=tickets` runs in CI.
- **AC14:** a sqlite-tagged CLI test holds the lock in-process and runs `test`.
- **AC15:** a testdata project with no `schema.yaml` on disk.
- **AC16:** a goroutine count before and after 100 cases (or `goleak` if it is already a dependency).
- **Clock guard:** the archguard scan fails on a newly added `time.Now()` in the write-path packages.
- **Dependency gate:** a CI step runs `go list -deps -tags postgres ./cmd/rela` and checks that bleve is absent.

**Edge Cases:**
- No `tests/` folder: the message "no tests found" and exit 0, or exit 1 with `--require-tests`.
- A file with no cases, or a Lua error outside `t.case`: the file fails, with its location.
- A fixture directory that is missing or empty.
- A fixture in an older schema shape: the case fails at load, naming the file and saying fixtures must match the current schema.
- An automation cascade that never ends: the cascade limit surfaces as a case failure.
- A cascade that fails partway leaves its earlier writes, because memstore has no rollback. This is documented as a memstore difference.
- `t.advance` with a negative duration is rejected.
- A generated value rejected by a rule the generator does not know: the error names the rule and the seed.
- Unicode in titles and principal names.

**Negative Tests:**
- An assertion failure reports file:line.
- A real HTTP or AI call fails the case.
- The VM cannot write files.
- An unknown type in `create` returns the manager's error verbatim.
- A role in `t.principal{roles}` that `acl.yaml` does not define is an error, not silently ignored.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- **A writer bypasses `Config.FS`** and writes under the root. Mitigation: AC6 on a project that uses every subsystem; route each writer found through `Config.FS`.
- **A full `prepare` per case is slow.** Mitigation: measure on `tickets/`. Above ~200 ms per case, add a per-assembly options variant of `SharedBase.Assemble` and reuse the parsed config.
- **memstore semantics differ from sqlite/postgres:** no `Tx` rollback, no version history, no refusal of undeclared types, and LinearSearch ranking. Mitigation: the guide lists every difference; operators who need their own backend get TKT-DAUS290.
- **Clock threading touches many packages.** Mitigation: the archguard scan and AC3 across every consumer.
- **Fixtures skip computed properties, `unique:`, version rows and data migrations.** Mitigation: documented as given state; `create` in a case uses the full write path.
- **New appbuild options widen the facade.** Mitigation: options only, no new `Services` fields.

Effort: xl (raised from l after the design review: clock threading, the FS split
and the principal model).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs-project: new guide "Testing your app with rela test". It covers generated data and `--seed`, principals and roles, given-state fixtures, memstore differences, and what is not reproducible.
- [x] docs/cli-reference.md: `rela test`
- [x] docs/lua-scripting.md: link to the test module reference

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 18 review-responses linked to TKT-0MCLX9S (4
critical, 9 significant, 4 minor, 1 nit). All are addressed in this plan.
