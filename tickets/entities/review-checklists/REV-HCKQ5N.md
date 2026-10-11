---
id: REV-HCKQ5N
type: review-checklist
title: 'Review: Piles: personal working sets of entities (service, API, scope source, UI)'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`go test -race ./...` green; two timing tests that hit the 10m package timeout or a wall-clock bound at load ~44 pass alone: dataentry TestAnalyzeProperties_StopsScanningAtCap, validation TestLuaValidation_PerRuleTimeout x3. Frontend 3821/3821. e2e piles.spec.ts 5/5)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues; arch-lint, plimsoll, lint-md clean)
- [x] Comment lint gate clean (`just comment-lint`) (no comment-report findings introduced by this diff)
- [x] Coverage maintained (`just coverage-check`) (package 50% and total 65% floors pass)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer in parallel; 18 findings)
- [x] All critical review-responses addressed (none from code review; 3 from design review addressed)
- [x] All significant review-responses addressed (RR-SEXBQP foreign push never evicts / MaxForeignPiles; RR-9NSDNW cross-process file lock)
- [x] Self-reviewed the diff for unrelated changes (every modified file is piles wiring, docs or tests)

**Review Responses:** RR-0IJ0DB, RR-1BN25K, RR-2RH309, RR-40SQ3S, RR-5AOBP7,
RR-5SKHIZ, RR-5U62D2, RR-5X4XBB, RR-9NSDNW, RR-B60JUO, RR-C905DM, RR-CONVVO,
RR-CWWQ40, RR-DLDXJT, RR-DP8E5K, RR-EEBC8T, RR-FIY3JM, RR-GK7I8A, RR-J4BUMY,
RR-KHFWMF, RR-KIBS1T, RR-KL4F1R, RR-M71X0F, RR-MJKTC4, RR-MTPE2X, RR-NVOXIC,
RR-O6M7J6, RR-P6QW0K, RR-QOSPDF, RR-SAYJ9F, RR-SEXBQP, RR-SEZ0XT, RR-TRYPDZ,
RR-V3Q5NA, RR-WBW5F4, RR-Z16WXF, RR-ZDMXNI, RR-ZGUO49. Code review: 2
significant and 9 minor/nit addressed, 2 minor deferred (RR-MJKTC4 kv sharding,
RR-1BN25K add/delete race), 1 nit wont-fix (RR-O6M7J6).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-5OGHJ5)

**Acceptance Status:**

- AC1, 2, 4 create, add, remove, order: PASS (pilestest on kv and pg; handler tests; live server: create 201, duplicate add 0, remove then count 2)
- AC3 concurrency: PASS (pilestest concurrent adds and creates on kv and pg; TestFileLocker_TwoStoresLoseNothing across two stores)
- AC5 step-through: PASS (scope tests; TestPiles_PositionAcrossFaces in a non-default world; Vitest; e2e [1/3] to [2/3])
- AC6 owner isolation: PASS (handler tests give a foreign id the same 404; live server 404 for unknown and malformed ids)
- AC7 hidden items: PASS (ACL fixture tests for GET, counts, position, export; byte-identical _remove)
- AC8 renames and deletes: PASS (pilestest; TestPiles_AssigneeLifecycle through entitymanager)
- AC9 actions: PASS (useListActions Vitest over mixed types)
- AC10 export: PASS (export tests; live server unknown transform 404)
- AC11 config validation: PASS (dataentryconfig table tests)
- AC12 limits and eviction: PASS (pilestest; foreign pushes never evict, RunKeepExistingTests)
- AC13 no owner: PASS (handler 403; piles_available false; Vitest)
- AC14 faces: PASS (pilestest faces; handler 409 ambiguous_address; Vitest address links)
- AC15 automation: PASS (engine and autocascade tests; TestPiles_CascadeWiring; TestPiles_AssigneeLifecycle)
- AC16 Lua and MCP: PASS (lua and mcp tool tests incl. header-read failure and read budgets)
- AC17 owner validation: PASS (TestPiles_PushOwnerCheckUsesCallerGate; service tests)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs` (DOCS-OW078N)
- [x] User-facing documentation updated (api-reference Piles section; data-entry, Lua, MCP, metamodel and postgres guides)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-OW078N

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A yet: nothing is committed; the commit waits for the user, who wants minimal messages: "feat: piles (TKT-K3RJLH)")
- [x] No TODOs or FIXMEs left unaddressed (none in the diff)
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: /pr runs after the ticket is done; PR and CI status are recorded on GitHub per TKT-UFV01M)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
