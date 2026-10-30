---
id: DOCS-9LXX9Q
type: docs-checklist
title: 'Docs: prose-check registry and engine (TKT-FUQL97)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (Vale glob matching against the absolute path, the code-point span mapping, why a position that does not fit is cleared, why the child runs in the temp dir)
- [x] Function/type docs if public API (check.Engine, Finding, Result, CheckerError; cmdexec.RunSpec, RunResult, RunWith, PurposeCheck; metamodel.CheckerDef, CheckableTypeName)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] CLAUDE.md updated (if new patterns) (rules-table row for checkers.md, package-table row for internal/check)
- [x] Help text accurate (if CLI changes) (rela analyze prose, analyze all --prose, rela-server --sandbox-check-read-paths)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (new docs/checkers.md; docs/metamodel.md, docs/cli-reference.md and the sandbox table in docs/transforms.md)
