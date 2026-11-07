---
id: TKT-7T5R9HV
type: ticket
title: 'rela test: config mutation testing'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Make small changes to the config (drop a grant, flip a `when:`, remove an
automation action, loosen a cardinality) and rerun the tests that depend on the
changed rule. Report surviving mutants: rules no test would notice changing.
Builds on impact analysis.
