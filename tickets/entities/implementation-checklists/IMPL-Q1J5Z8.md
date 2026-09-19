---
id: IMPL-Q1J5Z8
type: implementation-checklist
title: 'Implementation: Commands and actions are unreachable on entity types that declare faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `TestBuildCommandEnvFace` (table-driven,
faced + faceless), `TestEntityToTable_CarriesFace` (table-driven), and eight
cases in `NextActionOffers.test.ts` covering the `action` branch.
- [x] Integration tests written (test full flow, not just units) — the
`EntityDetail.world.test.ts` cases mount the real component against a faced view
response and assert the address handed to `CommandModal`, which is the seam the
bug lived in.
- [x] Happy path implemented — three changes, below.
- [x] Edge cases from planning handled — faceless types unchanged everywhere
(`RELA_ENTITY_ID` stays bare, `face` is `''`, the bare-id command test);
`confirm` declined runs nothing; a script `redirect` is followed; a failed
action leaves the page intact.
- [x] Error handling in place (errors surfaced, not swallowed) — a script error
opens the shared dialog with file:line and correlation id, anything else is a
toast. Same precedence as `useListActions`.

## Test Quality

- [x] Using fixture builders or factories for test data — reused the file's
`entry()` / `viewResponse()` / `standIn()` builders and `mountOffers`.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end — **by test, not by running a faced
server.** Recorded plainly: the gate is a pure computed and the env block is a
pure function, so both are decided by the assertions above rather than by
observation. A faced PostgreSQL instance was not stood up.
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified — as above, by test.

**Verification Evidence:**

- `go test ./...` — all packages pass (only pre-existing macOS linker warnings).
- `npm run test:run` — 185 files, 2940 tests pass (up from 2930).
- `npm run typecheck` — clean. `npm run lint` — 0 errors, 130 warnings, all
pre-existing and none in the touched files.
- `just arch-lint` — OK, no warnings. `just comment-lint` — clean.
- `just coverage-check` — package floor and total both PASS (79.7%).

**One planning assumption was wrong, and the fix is smaller for it.** The plan
said to NARROW the gate to `context: view`, on the reading that a view command
still receives default-world content. `viewworld.go:150` refutes it:

```go
if w.isDefault() || entry.Explicit {
```

An explicit `ID@face` is served literally under EVERY world, and the resolved
face is additionally ACL-checked by `faceReadable`. So both command contexts
already honour the address. The gate's whole premise — that the script gets
content other than what is on screen — was true only because this component
discarded the face before sending it. The gate is therefore **deleted** rather
than narrowed.

## Quality

- [x] Code follows project patterns (check similar code) — the action handler
mirrors `useListActions` / `Sidebar.vue` (script-error dialog precedence,
`message_type` toast, `redirect` push).
- [x] Checked for DRY opportunities — exported `reload` on `useNextAction`
rather than duplicating `load()`; reused `getScriptError`, `getErrorMessage` and
the existing confirm singleton. Did not extract a shared "run-action-and-report"
helper: the three call sites differ in what they do after (clear selection /
re-resolve slot / nothing), so the shared part is one `await` and a `catch`.
- [x] No security issues introduced — the face travels as an address the
server already parses and gates per face; no new endpoint, no new parameter.
`entity_type` is deliberately NOT sent (`BUG-ZWTDH9`: a caller-supplied type is
forgeable and must never be authorized against).
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

## Changes

1. **`EntityDetail.vue`** — gate deleted; `CommandModal` receives `servedRef`
(the address) instead of `bareEntityId`.
2. **`commands.go`** — `RELA_ENTITY_FACE` and `RELA_ENTITY_REF` added.
`RELA_ENTITY_ID` stays bare, so a face-unaware script is untouched.
3. **`runtime.go`** — `EntityToTable` exposes `face`; `id` stays bare.
4. **`NextActionOffers.vue`** — the `action` branch renders and runs, honouring
`confirm`. `set` still does not: it has no endpoint, so a button would have
nothing to call.
5. **Docs** — the command env table gained both variables and a faced example;
`content-states.md` now splits create from update, since update via a fused
address works and the old wording implied it did not.
