---
id: TKT-1VAVZ3O
type: ticket
title: Archguard allowlist for unpaged list reads
kind: enhancement
priority: high
effort: s
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

## Description

Pin every unpaged `ListEntities` / `ListRelations` / `ListEntityHeaders` call in
non-test code (outside the store packages) to an allowlist in
`internal/archguard`, like `bareref_test.go`. Each entry gets a reason and a
category: `batch-job` (migrations, analyze, import, export: reading the whole
graph is the job) or `request-path` (debt). The list may only shrink. About 211
call sites today.

## Done when

- The test passes on develop and adding a new unpaged call fails it.
- Every entry has a category and a reason.
