---
id: BUGA-YSZMFSM
type: bug-analysis-checklist
title: 'Analysis: rela migrate flattens folded block scalars in project YAML'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

## Notes

- Reproduced with a copy of the GRC demo config in /tmp (demo project not
  touched): restored a folded `description: >` in acl.yaml and removed the
  `scheduler-system` grant. The develop build of `rela migrate` writes the
  paragraph on one line followed by a blank line. Unit test
  `TestApply_KeepsFoldedScalarsFolded` fails on develop (149-char line, stray
  blank line).
- Environment: macOS, fs backend, yaml.v3 v3.0.1. Backend does not matter;
  this is the config rewrite.
- Literal blocks (`|`) are kept as written. Only folded blocks are affected.
- Related: `internal/migration/runner.go` is the only YAML encoder in the
  migration package. Configure (`internal/configedit`) writes YAML separately
  and was not part of this report.
