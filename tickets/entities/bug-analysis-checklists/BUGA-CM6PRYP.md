---
id: BUGA-CM6PRYP
type: bug-analysis-checklist
title: 'Analysis: Flow runs are missing from tenant data export'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Reproduction

- [x] Bug reproduced in a test: `TestMarkdownData_FlowRunsRoundTrip` exports and re-imports a project with an unfinished, branched run and checks its state.
- [x] Minimal reproduction steps documented: project with a flow and one open run, `rela db dump`, then `rela db import-fs`.
- [x] Environment/conditions noted: the sqlite `db dump`, `db load` and `import-fs` paths, which are the only export and import paths.

## Root Cause

- [x] Immediate cause identified (why1): `ExportMarkdownData` and `copyData` copied entities, relations and attachments only.
- [x] Contributing factors found (why2-3): the flow run store is a separate store outside the graph, added after the export code.
- [x] Systemic cause explored (why4-5): no check that the export covers every project store (measure AM-export-covers-every-project-store).

## Fix Planning

- [x] Fix approach determined: `flowrun.CopyRun` copies a run with its definition; the export writes runs into the KV run store under `.rela/`; `import-fs` and the in-transaction `db load` read them back.
- [x] Regression test planned: `TestMarkdownData_FlowRunsRoundTrip` (sqlite tag) and `TestCopyRun_RemovesPartialCopy`.
- [x] Related areas checked for similar issues: TKT-TWLN8UB (`db load` into existing sqlite) fixed in the same change.
