---
description: Assertions in runCmd() that panic if directory is empty or missing, plus explicit GIT_DIR env var
id: git-test-dir-assertions
kind: test
location: internal/git/git_test.go:runCmd()
status: active
title: Git test directory assertions
type: automated-measure
---

Defensive assertions in git test helper to prevent repo pollution:

- Panic if dir is empty
- Panic if dir does not exist
- Set GIT_DIR explicitly to prevent parent repo access
