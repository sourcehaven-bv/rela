---
id: TKT-REUP5S
type: ticket
title: Automations in an included schema file are silently dropped
kind: chore
priority: high
effort: s
status: backlog
---

## Description

`internal/metamodel/include.go`'s `partialMetamodel` has no `Automations` field,
so an `automations:` block in a file pulled in via `includes:` is **silently
discarded**. No error, no warning — the automations simply never run.

Verified empirically: a `schema.yaml` with `includes: [inc.yaml]`, where
`inc.yaml` declares one automation, loads successfully. The returned file list
confirms `inc.yaml` **was** read, and `len(meta.Automations)` is **0**.

`partialMetamodel` merges `Types`, `Entities`, `Relations` and `Validations`,
and has an explicit "fields that are not allowed in included files" group
(`Version`, `Namespace`, `Description`) that produces a load error.
`Automations` is in neither group, so it falls through the gap: not merged, and
not rejected.

## Impact

Silent non-execution is the worst failure shape for an automation. An operator
who splits a large schema into includes — the reason `includes:` exists — loses
every automation in the included files with no signal. Downstream, an audit log
missing those writes is indistinguishable from an automation that legitimately
did not fire.

## Approach

Either merge `Automations` like the other collections, or add it to the
not-allowed group so it fails loudly. Merging is the evident intent; whichever
is chosen, the silent-drop path must go. A regression test should assert an
automation declared in an included file is present in the loaded metamodel.

Found while reviewing TKT-JJRVX9 (per-automation audit attribution).
