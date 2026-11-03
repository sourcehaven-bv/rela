---
id: IMPL-H86VRVI
type: implementation-checklist
title: 'Implementation: rela migrate flattens folded block scalars in project YAML'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: refoldFolded has no error path; a decode mismatch falls back to the encoder output)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: test inputs are short YAML literals)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no entity objects)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `TestApply_KeepsFoldedScalarsFolded` runs `Apply` on a schema file that
  triggers the id_type migration: fails on develop (149-char line, stray
  blank line), passes with the fix. `TestRefoldFolded` covers wrapping,
  sequence entries, more-indented lines, keep chomping, unbreakable words,
  double spaces, literal blocks, and the fallback when a value would change.
- End to end on a copy of the GRC demo config in /tmp/foldproj (demo project
  not touched): `acl.yaml` with a folded `description: >` and no scheduler
  grant. Develop `rela migrate` writes the paragraph on one line plus a blank
  line. The fixed build keeps the five wrapped lines and adds the grant.
  Lines over 80 columns: 5 before, 5 after (all plain scalars, untouched).
- `go test ./internal/migration/ ./internal/cli/` pass; golangci-lint 0
  issues; `just arch-lint` OK; commentlint gate clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: the fallback is deliberate: the value is kept and the file is written as before the fix)
- [x] No debug code left behind
