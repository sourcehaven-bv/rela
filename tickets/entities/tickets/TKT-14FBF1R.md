---
id: TKT-14FBF1R
type: ticket
title: View section permission and hiding sections with only redacted fields
kind: enhancement
priority: medium
effort: s
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

An entity page section cannot be gated by permission, and an entry properties
section whose every field is hidden from the reader by `visible:` still shows
its heading with empty labels. A customer portal (harbourmaster) needs both:
staff-only sections, and sections that vanish when a customer may see none of
their fields.

## Scope

- Optional `permission:` on a view section. A UX filter like `permission:` on
navigation entries and dashboard cards; field values stay protected by
`visible:`. Refused on side-panel sections, which never omit one.
- The server drops an entry `display: properties` section when every field is
a property withheld from the reader. A relation field counts as content.
- Tests: Go table tests, config tests, a Playwright spec with a role that
cannot see a section's fields. Data-entry guide updated.
