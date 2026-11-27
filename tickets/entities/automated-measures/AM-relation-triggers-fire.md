---
id: AM-relation-triggers-fire
type: automated-measure
title: 'Test: relation_created and relation_removed triggers fire'
description: Asserts that relation writes fire their triggers, record the rule in the audit log, fail the write when a set fails and stop at the depth budget.
kind: test
location: internal/entitymanager/relation_triggers_test.go (TestRelationTriggers_*)
status: active
---
