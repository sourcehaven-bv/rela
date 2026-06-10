---
id: audit-durable-write-before-cascade-test
type: automated-measure
title: 'Test: entity audit reflects durable write even when cascade/upsert fails'
description: 'Regression for BUG-WXFZO6: a failingUpdateStore forces the post-automation re-write to error after the initial durable create; the test asserts exactly one create-entity audit record is still emitted. Fails if recordEntityAudit is moved back after Cascade.Process.'
kind: test
location: internal/entitymanager/audit_durability_test.go (TestCreate_AuditsDurableWriteWhenPostAutomationUpsertFails, TestUpdate_AuditsBeforeCascade)
status: active
---
