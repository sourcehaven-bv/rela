---
id: TKT-VMCQ2HO
type: ticket
title: Additive-only check for the wire schema
kind: test
priority: medium
effort: s
status: backlog
---

## Description

Native clients stay installed for months, so a renamed or removed wire field
breaks real users. Snapshot the schema of the generated `apiwire/v1` types and
fail CI when a field is removed, renamed or changes type. Adding fields stays
free. An intended break needs an explicit snapshot update in the same PR.

Part of FEAT-QRI3CFR (step 0.2).
