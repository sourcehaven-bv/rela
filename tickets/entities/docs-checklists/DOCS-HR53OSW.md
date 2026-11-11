---
id: DOCS-HR53OSW
type: docs-checklist
title: 'Docs: TKT-UH2Q9MN'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why a pushed value becomes the field's base (echo stop), why an uncertain create is held and not resent, why refused entries drop on a local edit, why resolve pre-checks outside the run lock
- [x] Function/type docs if public API: godoc on `Pusher`, `Remote.Push`, `Conflict`, `CheckResolve`, `Resolve`, `httpremote.Remote.Send`, `scheduler.RemoteConflicts` and `scheduler.ResolveRemote`, with `Nil:` contracts

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela remote conflicts` and `rela remote resolve` help, and the new report lines of `rela remote pull`

## User-facing Documentation

- [x] `docs/remote-sync.md`: "Syncing with a remote", sections "Two-way sync" and "Conflicts", the `push`, `share` and `conflict` keys, state and security additions
- [x] Security guide: `GUIDE-acl-security.md` "Pushes (`push: true`)" subsection; `docs/acl-security.md` regenerated with `just docs`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the push uses the existing data-entry write API unchanged)
