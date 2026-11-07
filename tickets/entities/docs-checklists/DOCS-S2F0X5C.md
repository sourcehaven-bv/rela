---
id: DOCS-S2F0X5C
type: docs-checklist
title: 'Docs: TKT-BC5G8RP'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the managed-type guard runs before the cascade and elevated skips, why managed incoming edges skip cascade authorization (RR-BLECTJJ), why each run gate branch keeps its endpoint type, why WithFlowRuns validates a deep copy
- [x] Function/type docs if public API: godoc on `entity.IsManagedName`, `metamodel.WithFlowRuns`, `audit.FlowStep`, `audit.WithFlowStep`, `audit.Stamp`, `audit.SourceOf`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: the `_` reservation is documented in the guides and godoc; no new cross-cutting coding rule)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI flag changes; `--run-as` refusal reuses the existing reserved-identity message)

## User-facing Documentation

- [x] `GUIDE-flows.md` (new Runs section: type, relations, who reads, who writes, conversions, comments, who-can, audit; alias `visible:` keys), `GUIDE-audit-log.md` (`flow`, `run`, `step`, `source` fields and a section on sources), `GUIDE-server-security.md` (`system:flow`, reserved names, canonical `visible:` keys); regenerated with `just docs`
- [x] Security guide: `GUIDE-server-security.md` updated as above

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no HTTP API change; runs are not served until TKT-9IT3LN8)
