---
id: IMPL-VMQB76
type: implementation-checklist
title: 'Implementation: Stop passing user-controlled IDs on the command line (use temp file)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test ./...` — pass; `golangci-lint` 0 issues; `just arch-lint` OK;
  `just plimsoll` OK; `just coverage-check` pass (76.9%).
- New regression tests in `document_noshell_test.go`:
  - `TestExecuteCommand_NoShellInterpretation` — runs a real command with the
    argument `a;b` + backticks + `$(id)` and asserts it arrives at the program
    UNINTERPRETED. This is the load-bearing one: reintroducing `sh -c` makes it
    fail.
  - `TestRenderEntityMarkdown_HostileIDIsInertData` — ids `-rf`, `-oevil`,
    `--output=x` survive into the {in} file verbatim as content, never as argv
    (acceptance criterion 2).
  - `TestRenderEntityMarkdown_CarriesID` — pins that frontmatter always carries
    `id:`, which is what makes {id} unnecessary.
  - `TestExecuteCommand_EmptyCommandIsRejected`.
- `TestValidateDocuments_LegacyIDPlaceholderRejected` (dataentryconfig) — a
  config using {id}/{id_lower} fails at LOAD with an error naming {in}, rather
  than silently passing the literal through.
- Serialized output inspected by hand — `id:` first, then `type:`, then
  properties in stable order, then the body. Matches the store's own file shape.

Two things surfaced during implementation that are NOT mechanical:

1. `cmdexec` sets no working directory, and `sh -c` previously ran with
   `cmd.Dir = projectRoot`. A relative program path (`command: "render.sh"`)
   therefore no longer resolves. Documented on the `projectRoot` field and in
   the migration note.
2. First attempt passed `cmdexec.WithTempDir(s.projectRoot)`, which broke a
   test against a read-only project dir — {in} is runner-owned scratch and
   belongs in the OS temp dir. Fixed.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
