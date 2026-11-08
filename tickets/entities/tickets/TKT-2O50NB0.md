---
id: TKT-2O50NB0
type: ticket
title: 'rela test: differential runs across rela versions and upgrade checks'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Compare traces and final state of the same suite on two rela binaries, to
surface behaviour drift that does not fail an assertion. Add `--strict` so
deprecation warnings fail the run, document `rela migrate config --check` as an
upgrade gate, and publish a CI action with a version matrix. Optional: an opt-in
canary corpus of operator suites run against `develop`.
