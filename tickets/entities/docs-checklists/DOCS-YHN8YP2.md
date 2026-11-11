---
id: DOCS-YHN8YP2
type: docs-checklist
title: Documentation
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (RlChannelComposer header and `requested` say when a kept-out type shows and when the request ends; decisionViews says why the type button is dropped)
- [x] Function/type docs if public API (MessagePresentation.Offer, ChannelMessagePresentation.offer and MessageFacts.HasPresetButtons are documented)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the data-entry guide is the place for data-entry.yaml keys)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] User guide updated (GUIDE-data-entry "Types that people do not post", regenerated into docs/data-entry.md)
- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PR titles)
- [x] ~~API docs updated (if applicable)~~ (N/A: `offer` rides on the existing presentation payload; no new endpoint)
