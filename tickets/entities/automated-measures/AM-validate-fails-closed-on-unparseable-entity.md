---
id: AM-validate-fails-closed-on-unparseable-entity
type: automated-measure
title: rela validate exits non-zero when an entity cannot be parsed
description: Test that a project containing one entity file with unparseable YAML frontmatter makes rela validate exit non-zero and name the offending file, rather than reporting success over a partial corpus.
kind: test
location: internal/cli/ (test to be written with the fix)
status: proposed
---
