---
id: BUGA-WE7U58B
type: bug-analysis-checklist
title: 'Analysis: Indented --- line closes frontmatter early and truncates properties'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (fuzz input 03748c2413d59a63 fails FuzzPropertyValuesTypeZoo on develop 03b51b014)
- [x] Minimal reproduction steps documented (go test -run='^FuzzPropertyValuesTypeZoo$' ./internal/store/fsstore with the seed; FormatDocument + ParseDocument of a value "a\n---\nb" also truncates)
- [x] Environment/conditions noted (fsstore and every caller of frontmatter.Split; any key or value whose block scalar has a "---" line)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (closing fence must start at column 0; trailing whitespace still tolerated)
- [x] Regression test planned (fuzz seed in fsstore testdata plus TestSplit cases)
- [x] Related areas checked for similar issues (callers in analysis, conflict, fsimport, markdown, templating, fsstore all use frontmatter.Split; their tests pass)
