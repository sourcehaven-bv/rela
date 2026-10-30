---
id: DOCS-TQC1GXX
type: docs-checklist
title: 'Docs: identity hint for acl_unstamped_principal'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (router.go: why the detail names the identity settings and marks the env var local-only; serve.go: why the check runs per generation)
- [x] Function/type docs if public API (warnUnidentifiedACL)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no flag or help text change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-server-security.md: new paragraph on acl.yaml refusing `unknown`, the error detail and the startup warning; GUIDE-acl-security.md: the fail-loud item points to the identity settings; regenerated docs/)
