---
id: DOCS-VCJAE5
type: docs-checklist
title: 'Docs: rela lease CLI (TKT-B610IP)'
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (runCLI globals; AmbiguousError wrapping)
- [x] Function/type docs if public API (lease.UserText, AmbiguousError, command help)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] CLAUDE.md updated (if new patterns) (Claiming tickets: unique label, lease id file with stdin, renew cadence, untrusted holder text)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-cli-reference.md regenerated into docs/cli-reference.md)
