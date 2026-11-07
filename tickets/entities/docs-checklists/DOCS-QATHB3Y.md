---
id: DOCS-QATHB3Y
type: docs-checklist
title: Documentation
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (presence.go, ownerAdmits and each fork edit carry a `rela fork` comment explaining the rule)
- [x] Function/type docs if public API (ConnectionConfig.StampAwareness and Owner are documented)

## Project Documentation

- [x] README updated (if applicable) (third_party/ygows/README.md records the upstream version, licence and every change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PR titles)
- [x] API docs updated (if applicable) (GUIDE-data-entry says the server sets the cursor label; docs/ regenerated)
