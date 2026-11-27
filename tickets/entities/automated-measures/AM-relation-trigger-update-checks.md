---
id: AM-relation-trigger-update-checks
type: automated-measure
title: 'Test: relation trigger writes run the update checks'
description: Asserts that a set written by a relation trigger is refused by a write-enforced relation rule and by strict validation, as PatchEntity refuses it, and that the relation write rolls back.
kind: test
location: internal/entitymanager/relation_trigger_checks_test.go
status: active
---
