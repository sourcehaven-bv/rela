---
id: DOCS-1B3QXW8
type: docs-checklist
title: 'Docs: TKT-0FMYPEZ'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious: the link edit check in `remotes_gate.go`, the checked field set on `remotesync.Target`, why `dataentrywire` requires an audited workspace, the page clamp in `RemoteConflictsView.vue`
- [x] Function/type docs if public API: godoc on `Target.Fields`, `Link`, `CheckLink`, `LoadLinked`, `RemoteStatuses` and the dataentry `RemoteService`; comment-lint `doclink` gate clean

## Project Documentation

- [x] README updated (if applicable): remote sync guide links
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing gating and aggregate rules)
- [x] Help text accurate (if CLI changes): `rela remote link` help and `rela remote conflicts` output

## User-facing Documentation

- [x] Remote sync guides split into GUIDE-remote-sync and GUIDE-remote-sync-two-way; data-entry guide has the Remotes page; regenerated with `just docs`
- [x] Security guide: `GUIDE-acl-security.md` threat model for the remote routes (row and field gate, aggregates after the gate, edit rights for take remote and link, no secrets on the wire, audit)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): `docs/data-entry/api-reference.md` remote routes; OpenAPI left out like the git routes, a browser surface
