---
id: BUG-95W7MV
type: bug
title: Tracer and analyze skip faced types
description: trace, find_path and analyze duplicates/unique/gaps/orphans query default rows only, so faced types are silently absent.
priority: medium
effort: m
status: backlog
---

## Problem

Reported by the face-awareness inventory, not yet verified. The tracer (`trace`,
`find_path`; `tracer.go:100,156,200`) and `analyze` duplicates, unique, gaps and
orphans (`analysis.go:149-304`) skip faced types or mishandle them, because they
query default rows only.

## Expected

Each analysis states which faces it covers (all faces, or a world) and includes
faced types. A faced type is never silently absent from a report.
