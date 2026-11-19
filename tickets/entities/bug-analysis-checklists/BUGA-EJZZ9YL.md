---
id: BUGA-EJZZ9YL
type: bug-analysis-checklist
title: 'Analysis: The Changes list shows customers internal records and raw names'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestEntityChanges_LeavesOutManagedNames, TestEntityChanges_HiddenUpdatesDoNotShortenAPage and the changed SummaryNamesNoHiddenProperty fail on develop; the new ChangesPanel tests fail on the old panel (no gate, raw op and names))
- [x] Minimal reproduction steps documented (as a customer, open an entity whose flow run added a _carries relation or whose hidden property changed: the Changes list shows the _carries record, a bare 'updated' and raw names such as update-entity and system:scheduler)
- [x] Environment/conditions noted (database builds (PostgreSQL, SQLite), where the Changes list exists; an ACL with visible: blocks and role_relations)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (the server leaves out _ relations, updates of only hidden or _ properties (before paging) and sends a titles map; the panel follows header_buttons.history and shows labels, titles, 'System' and the trigger kind)
- [x] Regression test planned (Go tests in visibility and dataentry, a budget test for the title batch, and ChangesPanel unit tests for the gate and the labels)
- [x] Related areas checked for similar issues (the history timeline already follows header_buttons.history and gates fields; the global audit view is for audit:read holders and keeps raw values on purpose)
