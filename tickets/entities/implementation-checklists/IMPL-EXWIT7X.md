---
id: IMPL-EXWIT7X
type: implementation-checklist
title: 'Implementation: Flows M5c: delete the old transition and automation paths and dogfood the tickets workflow'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (transitions_show_test.go: accessors, nil safety, Mermaid, prefix; cli TestFlowShow_Headless)
- [x] Integration tests written (test full flow, not just units) (the moved statemachine suites run unchanged in flowcfg; entitymanager, affordances and dataentry suites exercise the enforcer through the wiring)
- [x] Happy path implemented
- [x] Edge cases from planning handled (nil Transitions, unknown headless name, bare type name, flows.yaml missing or broken, unset status refusal)
- [x] Error handling in place (errors surfaced, not swallowed) (show returns the flows.yaml error after printing headless flows)

## Test Quality

- [x] Using fixture builders or factories for test data (compileCLIHeadless builds the metamodel once)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (flow names built from HeadlessPrefix)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Scratch copy of tickets/ with the branch binary: a ticket walked backlog to done and a bug walked backlog to done. Every transition created the expected checklist and relation, and started and completed were set. A checklist moved to done got its completed date.
- `rela flow show` and `rela flow validate` on a copy of the dataentry fixture list and render `schema.ticket_status` as text, JSON and a Mermaid diagram.
- `go test -race` passes on flowcfg, cli, appbuild, entitymanager, affordances, dataentry, attachment, remotesync, comments, acl, metamodel, automation, autocascade and archguard. `go vet` passes on all build tags.
- `just arch-lint`, lint, comment-lint, plimsoll and agentrules pass. archplan check has no new finding beyond the base; the baseline records the moved TransitionVerdict synonym and the flowcfg split with reasons.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (CompileHeadlessFlows is the single compile entry for boot, schema check, wiring and the CLI)
- [x] No security issues introduced (enforcer behaviour unchanged; guard posture unchanged)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
