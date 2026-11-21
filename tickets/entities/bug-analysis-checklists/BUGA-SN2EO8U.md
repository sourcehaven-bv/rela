---
id: BUGA-SN2EO8U
type: bug-analysis-checklist
title: 'Analysis: Failed flow runs hide their error and blocked steps give no field feedback'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (harbourmaster signup flow in the saas-dev test environment)
- [x] Minimal reproduction steps documented (in the bug description)
- [x] Environment/conditions noted (a job step whose webhook fails; a customer and a manager user)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (keep the redacted last error on the run for managers; failed runs on Flow tasks; rule fields and pattern warnings at the step form's fields; `failed_message:` and `title:` in flows.yaml)
- [x] Regression test planned (flowengine, flowcfg, writerule, appbuild and dataentry Go tests; FlowRunCard, FlowLists and FieldShell vitest tests)
- [x] Related areas checked for similar issues (the entity form already shows field errors inline; MCP and the next-action source leave the failed list out)
