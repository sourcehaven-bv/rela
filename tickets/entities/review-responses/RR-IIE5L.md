---
id: RR-IIE5L
type: review-response
title: 'Deferred: NewFSFactory exists but production uses struct literal; AC6 is ''decorative'''
finding: 'cranky-code-reviewer #6: added a constructor that validates non-nil inputs, but the single production call site (workspace.go:201) still uses &app.FSFactory{FS: fs, Paths: paths, UserState: us}. The constructor is unused.'
severity: significant
reason: Switching to the constructor requires unexporting FSFactory's fields, which ripples into test files (factory_test.go uses struct literals directly). Leaving the exported struct with a documented non-nil UserState expectation is acceptable for now; enforcement lives in factory.loadEncryption's nil-check rather than the constructor. Tracked as a cleanup follow-up.
status: deferred
---
