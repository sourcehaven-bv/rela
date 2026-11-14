---
id: DOCS-8EI1J3V
type: docs-checklist
title: 'Docs: Flows M5c'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (flowcfg doc.go covers both kinds of flows and the unset rule; flows.go Run explains the JSON and text behaviour on a broken flows.yaml)
- [x] Function/type docs if public API (HeadlessFlow accessors, Transitions.Headless/HeadlessFlow/Uses, HeadlessPrefix, appbuild.CompileHeadlessFlows and BuildTransitionWiring)

## Project Documentation

- [x] ~~README updated~~ (N/A: the README does not describe rela flow or transitions)
- [x] CLAUDE.md updated (if new patterns) (.claude/rules/predicate.md now points at internal/flowcfg/transitions*.go; root CLAUDE.md has no statemachine mention)
- [x] Help text accurate (if CLI changes) (rela flow validate and show help name the headless flows and the schema.<enum type> form)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file)
- [x] API docs updated (if applicable) (GUIDE-flows, GUIDE-metamodel and GUIDE-cli-reference in docs-project, regenerated into docs/ with just docs)
