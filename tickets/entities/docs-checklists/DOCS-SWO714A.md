---
id: DOCS-SWO714A
type: docs-checklist
title: 'Docs: rela test v1 (2/4): injectable clock and determinism seams'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (seam precedence in lua/seams.go; why audit and version stamps stay on the wall clock; the GC clock; the G704 trust boundary)
- [x] Function/type docs if public API (WithClock, WithMailTransport, WithHTTPClient, WithAIProvider, WithOutputDir, ErrRealNetwork, SharedBase.QueryScopes, lua.Seams, lua.WithHTTPClient, automation.WithClock, writerule.WithClock, memstore.WithClock, entitymanager.Deps.Clock; NewInMemory godoc lists the safe defaults and their boundary)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: Go-level options with no user-facing surface yet; the rela test command and its docs land in parts 3 and 4)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing options and archguard allowlist patterns; the wall-clock guard explains itself in its failure message)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PRs)
- [x] ~~API docs updated (if applicable)~~ (N/A: no HTTP API changes)
