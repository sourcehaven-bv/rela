---
id: DOCS-CDV002
type: docs-checklist
title: 'Docs: declarative caldav: collection config'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `CalDAVConfig` (why the `static:`/`dynamic:` split exists before its second member does), `CalDAVCollection` (one collection = one entity type = one symmetrical mapping), `CalDAVCompletion`, `CalDAVPriorityMap` and `CalDAVOnDelete`
- [x] Rationale recorded on `validateCalDAVCompletionReachable` for why a `where:` clause excluding the completed value is rejected at load — it produces the silently-reverting checkbox, the single most baffling CalDAV symptom
- [x] Godoc on `read_only:` explaining it as a CONTAINMENT control, orthogonal to ACL

## Project Documentation

- [x] `docs/caldav.md` — the deployment guide: config shape under `caldav.static:`, the field-mapping table, `priority_map:` bucketing, `description: body`, `read_only:`, deletion semantics and constraints
- [x] `docs/caldav-clients.md` — per-client compatibility table, rich-text handling, and what each client does with a refused write

## External Documentation

- [x] ~~README~~ (N/A: covered by the two docs/ guides, linked from the feature entry)

**Docs verified:** the `caldav.static:` example in `docs/caldav.md` matches the
shape the validator accepts (checked against the running demo project).
