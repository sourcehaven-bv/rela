---
id: deleteentity-failsecure-test
type: automated-measure
title: 'Test: DeleteEntity is fail-secure on relation-cleanup I/O errors'
description: 'Regression tests for BUG-2W3AJ. TestDeleteEntity_RelationRemoveError_FailsSecure (fsstore) injects a relation-file Remove failure via storage.ErrorFS and asserts the cascade delete errors and leaves both entity and relation in place (never orphaned). TestDeleteEntity_PropagatesStoreError and TestDeleteEntity_CascadeAuditsReportedRelations (entitymanager) assert the Manager surfaces a store delete error rather than swallowing it, and that a successful cascade emits one delete-relation audit record per reported relation plus the delete-entity record.'
kind: test
location: internal/store/fsstore/recovery_test.go (TestDeleteEntity_RelationRemoveError_FailsSecure) + internal/entitymanager/manager_delete_test.go (TestDeleteEntity_PropagatesStoreError, TestDeleteEntity_CascadeAuditsReportedRelations)
status: active
---
