---
id: create-no-overwrite-test
type: automated-measure
title: 'Test: entity create never overwrites on ID conflict'
description: 'Regression for BUG-R2PV8G: a conflictOnCreateStore returns store.ErrConflict from CreateEntity and counts UpdateEntity calls. The test asserts CreateEntity returns ErrEntityAlreadyExists with zero UpdateEntity calls — proving the create path never falls through to an overwrite. Fails if createCore reverts to writing through upsertEntity.'
kind: test
location: internal/entitymanager/create_conflict_test.go (TestCreate_ConflictDoesNotOverwrite)
status: active
---
