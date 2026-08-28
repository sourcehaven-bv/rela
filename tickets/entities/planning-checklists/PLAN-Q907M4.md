---
id: PLAN-Q907M4
type: planning-checklist
title: 'Planning: fail-closed data-entry script read wiring'
status: done
---

## Understanding

- [x] Problem understood — rela#1198 (IB review, CONTROL-5-15): `scriptReader`/`scriptTracer` fall back to the raw ungated store on a gate construction fault.
- [x] Issue claims verified against code rather than accepted. Two of three premises were refuted (MCP not served by this wiring; branches unreachable), and the genuine justification (the unattended IdP webhook) was identified independently.
- [x] Scope clear — wiring change only; `DenyReader`/`DenyTracer` already exist.

## Research

- [x] ~~Library search~~ (N/A: no new dependency; the fail-closed machinery is already in-tree from RR-GKCZO5)
- [x] Existing pattern reused — `appbuild.scriptEntityReader`/`scriptTracer` are the direct precedent.
- [x] All consumers of `App.luaWriteDeps` enumerated (actions, export_render, webhook).

## Approach

- [x] Chosen: make all three consumers fail closed rather than splitting per-consumer policy. An interactive caller receiving `ErrReaderUnavailable` gets the loud, immediate outage the fail-open argument actually wanted, without the ungated read.
- [x] NopACL path explicitly preserved — absence of `acl.yaml` is an intentional state, not a fault.

## Security

- [x] Input trust — no new input surface; this narrows an existing read path.
- [x] Failure direction — fail-closed on fault; deny remains distinct from unavailable.
- [x] Considered the counter-risk (breaking policy-less deployments) and pinned it with a dedicated test.

## Test plan

- [x] Fail-closed reader, fail-closed tracer, policy-less-stays-unrestricted.
- [x] Each test to be mutation-verified rather than merely passing.
