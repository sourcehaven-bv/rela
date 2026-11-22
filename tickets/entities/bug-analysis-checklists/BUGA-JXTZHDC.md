---
id: BUGA-JXTZHDC
type: bug-analysis-checklist
title: 'Analysis: GET /_flows answers 404 on every page of a project without flows'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestFlowsAPI_NotWired pins the 404 without flows; useFlowList called listFlows unconditionally, so the new vitest "does not ask for the list" fails on develop)
- [x] Minimal reproduction steps documented (open any page of a project without flows.yaml with the browser console open)
- [x] Environment/conditions noted (every backend; only projects without flows.yaml)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (advertise the surface in /_schema as capabilities.flows, like capabilities.git, and gate useFlowList on it; chosen over a 200 with an empty list because the other flow routes would still answer 404 and the SPA would still make a request on every page)
- [x] Regression test planned (Go: TestSchemaCapabilities_Flows; vitest: useFlowList skips the call without flows and waits for the schema)
- [x] Related areas checked for similar issues (piles are advertised through piles_available and git through capabilities.git; records, leases and comments are not called on every page)
