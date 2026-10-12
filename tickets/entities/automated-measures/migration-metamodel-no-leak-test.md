---
id: migration-metamodel-no-leak-test
type: automated-measure
title: 'Test: a migration run without a metamodel does not inherit one from an earlier run'
description: 'Regression for BUG-MRNWK4: runs data-entry detection with a metamodel and then without one and asserts the second run takes the no-metamodel path. Fails if the runner binds the metamodel by mutating the registered migration singletons again.'
kind: test
location: internal/migration/runner_test.go (TestDetectFromNodeWithMetamodel_DoesNotLeakMetamodelAcrossRuns)
status: active
---
