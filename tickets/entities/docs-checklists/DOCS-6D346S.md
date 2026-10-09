---
id: DOCS-6D346S
type: docs-checklist
title: 'Docs: OAuth token binding and Basecamp reference connector'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not cover connectors; examples/basecamp has its own README)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: token store follows the existing state.KV, lock and capability patterns)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog)
- [x] API docs updated (if applicable)

Guides updated: Lua scripting (OAuth tokens, tier table, encode_query,
retry_after, integration: principals in the sync section), CLI reference (rela
token, validate checks connections.yaml), scheduled tasks, ACL security, server
security, desktop (Connections section); docs/ regenerated.
