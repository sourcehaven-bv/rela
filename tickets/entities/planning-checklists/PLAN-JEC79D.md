---
id: PLAN-JEC79D
type: planning-checklist
title: 'Planning: Prose-check registry and engine with analyze prose'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

This is the first of three PRs split from TKT-EKY2TV. The full design,
alternatives and design review are in that ticket's planning checklist; this
checklist covers the part that ships here.

**Scope:**

In: the `checkers:` registry, type-name allowlist, include rejection,
Configure lock; cmdexec `PurposeCheck`, `RunWith` and the temp working
directory; `internal/check` with the Vale and native parsers; `rela analyze
prose` and `rela analyze all --prose`; docs.

Out: the `_check` endpoint, Milkdown decorations, MCP `prose`, batch mode,
dismissing findings.

**Acceptance Criteria:**

1. A `checkers:` entry with an unknown or missing `format`, an empty
   `command`, an exit code outside 0-255 or a timeout outside (0, 60s] fails
   the load naming the checker. So does `checkers:` in an include, and a type
   name outside `^[A-Za-z0-9][A-Za-z0-9_-]*$` while checkers are declared.
   Test: `internal/metamodel/checkers_test.go`.
2. The checker gets the body at `<type>/<id>.md` and `{type}` substituted; an
   unsafe id becomes `body.md`. A `[**/<type>/*.md]` Vale section fires on the
   real path and not for other types. Test: `TestCheck_InputNameAndTypePlaceholder`,
   `TestCheck_UnsafeIDFallsBack`, `TestCheck_ValeTypeSections` (real Vale).
3. Vale and native output parse into the same findings, with replacements and
   code-point positions. Test: `parse_test.go` with a golden file from Vale
   3.25.1 including an emoji.
4. An exit code in `exit_codes` returns findings; any other exit, malformed
   output or a timeout is an error for that checker only. Test:
   `TestRunWith_AcceptedExitCode`, `TestCheck_OneFailingCheckerDoesNotHideOthers`,
   `TestCheck_Timeout`.
5. `rela analyze prose` lists findings for `analyze: true` checkers only,
   per entity, in text and JSON, and exits non-zero when a checker failed.
   `analyze all` runs no checker without `--prose`. Test:
   `internal/cli/analyze_prose_test.go`.
6. The Configure space refuses edits to `checkers:`. Test:
   `TestAllowlist_ClassifiesEveryKey`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: see TKT-EKY2TV's planning; prototyped with Vale on 76 documents)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Vale (external command). Embedding it was rejected:
its packages are all under `internal/`, it needs cgo (tree-sitter) while
rela builds with `CGO_ENABLED=0`, and it runs helpers and Tengo scripts that
would escape the sandbox in-process. Precedents: the `transforms:` registry
and engine, per-purpose sandbox read paths, `IncludeHasRootFieldError` for
`worlds:`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** as in TKT-EKY2TV's planning, items 1-4, 6 (CLI part)
and 8. `internal/check` depends on `cmdexec` and `metamodel` only; `cli` may
import it.

**Alternatives rejected:** a resident Vale process (Vale has no server mode,
and a shared process breaks per-run isolation; measured cost is startup, so
batching is the fix); copying project config into a staging dir (checker
config belongs to the operator).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

Commands come from the metamodel only. Type names and ids are allowlisted
before they become a path, and cmdexec checks the input name again. The CLI
checks `CheckProjectNotExposed` before running. Checkers have their own read
list and a separate pool. Output is capped and parsed strictly.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

Integration: inline `sh`/`awk` fake checkers run through the real sandboxed
runner, plus a Vale-gated test. Edge cases: empty body, CRLF, diacritics and
emoji, positions outside the body, unsafe ids, cancelled context.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

Analyze over many entities costs one process each (measured 31 ms per file on
76 documents): opt-in per checker and per run; batch mode is the follow-up.
Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/checkers.md (new)
- [x] docs/metamodel.md: the `checkers:` key
- [x] docs/cli-reference.md: `rela analyze prose`
- [x] docs/transforms.md: the check read-path row
- [x] CLAUDE.md and `.claude/rules/checkers.md`
- [x] ~~README.md~~ (N/A: no README change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the review on TKT-EKY2TV's plan applies; findings
1, 3, 4, 7, 8, 9, 11 and 12 concern this PR and are implemented here.
