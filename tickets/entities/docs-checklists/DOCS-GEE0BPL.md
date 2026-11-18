---
id: DOCS-GEE0BPL
type: docs-checklist
title: 'Docs: Audit service seam and SQL audit log'
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] README updated (if applicable)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting pattern; the sink follows the realtimeLocal reassembly pattern)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog file; release notes come from PR titles)
- [x] API docs updated (if applicable)

GUIDE-audit-log (database audit log, hash chain, threat model, roles, delivery,
import, retention) and GUIDE-cli-reference (`rela audit`) were updated and `just
docs` regenerated `docs/audit-log.md` and `docs/cli-reference.md`. No HTTP API
changes in this PR.
