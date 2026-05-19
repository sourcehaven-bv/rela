---
id: conflict-marker-line-anchor-test
type: automated-measure
title: 'Test: git-conflict-marker detector requires line-start anchoring'
description: 'Unit test loading a markdown file whose body contains the substring `<{7}` inside a code span or quoted prose (NOT at column 0). Asserts the loader treats it as a normal markdown file with no errors. Regression for BUG-WN6D: today the loader skips the file as `unresolved git conflicts`, silently masking validation against that entity.'
kind: test
location: internal/store/fsstore/* (TBD by implementer)
status: proposed
---
