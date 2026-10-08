---
id: AM-hidden-write-outcomes
type: automated-measure
title: 'Regression tests: write errors and counts reveal nothing about hidden data'
description: Asserts that a cascade denial over a relation the caller cannot see names neither the relation nor its endpoints (faced tails included), that a visible denial wins over a hidden one, that rename counts only visible relations, that rename is refused for generated ids, and that a rename onto a hidden id reveals only the collision. Catches any write path that reports raw-store detail to a caller (the class that produced BUG-1BXQDD).
kind: test
location: internal/entitymanager/hidden_write_errors_test.go
status: active
---
