---
id: PLAN-AN1FYH4
type: planning-checklist
title: 'Planning: rela test v1 (2/4): injectable clock and determinism seams (mail transport, HTTP client, AI provider, output dir)'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Part 2 of TKT-0MCLX9S. The full design is PLAN-G8YMBDB (Scope,
Approach steps 2-3). This ticket adds the appbuild options `WithClock`,
`WithMailTransport`, `WithHTTPClient`, `WithAIProvider` and `WithOutputDir`,
threads them through the write path, and makes `NewInMemory` safe by default. It
also closes RR-8ULC8I2 and RR-5PV25TX from TKT-J4HMZ2B.

Out of scope: `rela test`, `rela.test`, fakers, principals, `internal/apptest`
(TKT-UO8UIGO); the tickets suite, CI and docs (TKT-E2IMTMG).

**Acceptance Criteria:**
1. With a fixed clock, `{{today}}` in an automation, `today()` in validation, writerule and query-scope predicates, a computed property, `rela.today()` in automation Lua and memstore mod_time all see the same instant. Test: one integration test in `internal/appbuild`.
2. Mail from automation Lua lands in the injected transport; a recipient blocked by `RecipientPolicy` stays blocked. Test: `mail.yaml` with SMTP and a recipients allowlist.
3. A refusing HTTP client and AI provider are used by automation Lua that declares those capabilities. Test: an automation calling `http.get` and `ai.chat` reports the refusal.
4. `write_file` with the option writes outside the project root. Test: the PR 1 tree-hash test gains a `write_file` automation; a second test checks the file lands in the given dir.
5. The guard test fails on a new `time.Now` in the scanned packages. Test: the scanner run on a synthetic source.
6. `NewInMemory` with no options sends no mail over the network, refuses HTTP and AI, reads no secrets or `ai.yaml`, and writes no files under the root; the temp output dir is removed on `Services.Close`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: design done in PLAN-G8YMBDB)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (PLAN-G8YMBDB)

**Existing Solutions:** Existing seams reused: `lua.WithClock`,
`predicatefns.NewEvaluatorWithClock`, `computed.CompileWithClock`,
`automation.TemplateVars.Now`, `lease.Clock`, `mail.NewLuaSender` (policy from
config, transport passed in), `mail.MemorySender`, `lua.WithAIProvider`,
`lua.WithOutputDir`, `appbuild.WithHostConfig`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- appbuild `options` gets `clock`, `mailTransport`, `httpClient`, `aiProvider`, `outputDir`. `SharedBase.now()` is the one clock source of a world.
- Lua: a new `lua.Seams` value on `ReadDeps` (clock, HTTP client, AI provider, output dir, mail loader). `newRuntime` applies it after the options, so a world's seams win over a per-call option. Every runtime built from Services deps (automations, cascades, actions, scheduled tasks, validation rules, CLI scripts) carries it. `lua.WithHTTPClient` is the per-runtime client option; the package-global client stays the default.
- Clock: `automation.WithClock`, `writerule.WithClock`, `entitymanager.Deps.Clock` (copy/convert interpolation, computed fallback, writerule), `memstore.WithClock`, validation reads `ReadDeps.Seams.Clock`, lease gets a clock adapter, `SharedBase.QueryScopes` builds the scope evaluator with the base clock.
- Mail: `startMailRuntime` uses the injected sender instead of `mail.SenderFor`, after parsing `mail.yaml`. Lua `mail.send` uses `mail.NewLuaSender(injected, cfg)`, so the policy and base URL still come from the file.
- `NewInMemory` defaults (each overridable by the caller's option): memory mail transport, refusing HTTP client, refusing AI provider, a temp output dir outside the root removed on Close, and a host config that serves `mail.yaml` only (no `ai.yaml`, no secrets).
- Guard: `internal/archguard/testclock_test.go` parses the scanned packages' non-test files and counts `time.Now` selectors per file against an allowlist with a reason and an exact count.

Decisions the umbrella plan left open:
- **Seams win over options** in a Lua runtime. Rejected: options winning, because a per-call `WithOutputDir` (CLI flag) could then point a world's writes at the project root.
- **No outbox flush hook.** The outbox has no producer today: `mail.send` in Lua and scheduled mail both send synchronously through the sender. A flush hook would be dead code; mail is visible in the transport right after the write. When the declarative layer starts enqueuing, it adds the flush.
- **Audit and version stamps stay on the wall clock** (allowlisted, godoc on `WithClock`).
- The guard counts `time.Now` as a call and as a function value, so `computed.Compile`-style defaults are visible too.

**Files to modify:**
`internal/appbuild/{appbuild.go,memrecipe.go,mail.go,scheduled_mail.go,queryscopes.go,leases.go,elevatedreads.go}`,
new `internal/appbuild/seams.go`; `internal/lua/{runtime.go,http.go,deps.go}`;
`internal/script/runtime.go`; `internal/automation/engine.go`;
`internal/validation/validation.go`; `internal/writerule/writerule.go`;
`internal/entitymanager/{manager.go,copy.go,copy_apply.go,conversion*.go}`;
`internal/store/memstore/*.go`; new `internal/archguard/testclock_test.go`;
tests in `internal/appbuild`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Host config (`mail.yaml`, `ai.yaml`, secrets) is
the input. `NewInMemory` serves only `mail.yaml` from it by default (allowlist
of one file); secrets and `ai.yaml` read as absent.

**Security-Sensitive Operations:** Outbound mail, HTTP and AI are replaced or
refused by default in `NewInMemory`. `write_file` goes to a temp dir outside the
root. A caller that passes an option takes responsibility for what it injects.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria. All are integration tests over
a `NewInMemory` world on a temp project, plus unit tests in `lua` (seams
precedence, per-runtime HTTP client), `memstore` (clock), `automation`,
`writerule`.

**Edge Cases:** no `mail.yaml` (mail stays not configured even with a
transport); caller-supplied output dir is not removed on Close; Close twice
removes the temp dir once.

**Negative Tests:** a blocked recipient; a refused HTTP call; a refused AI call;
a new `time.Now` in a scanned file.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A runtime built outside Services deps (data-entry app, docs) does not
carry the seams. Mitigation: those surfaces are not part of a `NewInMemory`
world; documented on `lua.Seams`. Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed (the guide ships with TKT-E2IMTMG); godoc on the options.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the umbrella design was reviewed under TKT-0MCLX9S, 18 review responses addressed in PLAN-G8YMBDB)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see TKT-0MCLX9S review responses.
