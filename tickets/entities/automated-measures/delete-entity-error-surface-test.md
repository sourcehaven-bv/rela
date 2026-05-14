---
id: delete-entity-error-surface-test
type: automated-measure
title: 'Test: DeleteEntity surfaces non-NotFound relation-delete errors'
description: When BUG-C20T is fixed, add a regression test that wraps the store with an injected non-NotFound failure on a relation delete and asserts (a) DeleteEntity returns the error, (b) DeletedRelations does not include the failed deletion, (c) the entity is NOT deleted on partial failure.
kind: test
location: internal/entitymanager/manager_test.go (added when BUG-C20T is fixed)
status: proposed
---
