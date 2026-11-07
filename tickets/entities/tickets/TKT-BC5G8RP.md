---
id: TKT-BC5G8RP
type: ticket
title: 'Flows M1c: _flow-run type, relations, ACL and provenance'
kind: enhancement
priority: medium
effort: l
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

## Description

Implicit rela-managed type _flow-run and relations _carries and _produced. Only
the engine principal writes them, compiled into the ACL allowlist. Visibility
follows the subject through related(_carries, ...). Audit entries carry flow,
run and step. Split from TKT-XHANLJU.

## Carried over from TKT-PV9R0GK review

- RR-SVDODML: refuse renaming a `_flow-run` entity. The run store keys a
run's state by the entity id, so a rename would orphan it.
