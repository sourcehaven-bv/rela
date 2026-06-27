---
id: collect-iterator-errors-test
type: automated-measure
title: 'Test: ID-gen and delete-safety scans fail on iterator errors'
description: 'Regression for BUG-R8ELTO: store stubs inject a terminal error into the ListEntities / ListRelations iterators. The tests assert auto-ID CreateEntity surfaces the entity-scan error (instead of minting a possibly-colliding ID) and DeleteEntity surfaces the relation-scan error (instead of under-counting the delete-safety gate). Both fail if collectAllIDs / collectIncidentRelations revert to swallowing iterator errors.'
kind: test
location: internal/entitymanager/collect_errors_test.go (TestCreate_FailsWhenIDScanErrors, TestDelete_FailsWhenRelationScanErrors)
status: active
---
