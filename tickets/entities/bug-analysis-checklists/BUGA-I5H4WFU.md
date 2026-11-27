---
id: BUGA-I5H4WFU
type: bug-analysis-checklist
title: 'Analysis: MCP update_entity ignores field-level write grants'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (the corpus recording showed the write allowed; a production-wired test through appbuild.New and RemoteMCPServer shows it refused, and fails when the MCP handle is swapped for the ungated one)
- [x] Minimal reproduction steps documented (TestRemoteMCP_UpdateEntityHonorsFieldGrants; corpus case BUG-VUNO11-mcp-update-denied-field)
- [x] Environment/conditions noted (fs backend; Declarative ACL with a `fields:` grant that leaves the written field out)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (export appbuild.BuildFieldWriteGate over buildFieldPolicy; the fixture uses it with its resolved ACL. Threat model in the bug description. Design review by go-architect: approved; adopted its findings to key the gate on the resolved ACL rather than cfg.declarative, keep it out of TransitionWiring, tag the nil contract, and name it BuildFieldWriteGate)
- [x] Regression test planned (TestRemoteMCP_UpdateEntityHonorsFieldGrants plus the re-recorded corpus)
- [x] Related areas checked for similar issues (other fixture authorization deps come from TransitionWiring or are required by entitymanager.New; consumers of the fixture with an ACL in internal/mcp, internal/appbuild and internal/dataentry were re-run)
