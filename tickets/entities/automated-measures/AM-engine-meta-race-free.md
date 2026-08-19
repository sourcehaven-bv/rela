---
id: AM-engine-meta-race-free
type: automated-measure
title: Engine.meta access is race-free under concurrent SetMetamodel
description: A test calling SetMetamodel concurrently with Process under -race. The current suite is race-clean only because nothing calls SetMetamodel concurrently; the regression test has to create that situation deliberately.
kind: test
location: internal/automation/ (test to be written with the fix)
status: proposed
---
