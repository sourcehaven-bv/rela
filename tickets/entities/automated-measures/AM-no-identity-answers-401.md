---
id: AM-no-identity-answers-401
type: automated-measure
title: 'Test: a data-entry request with no bound identity answers 401 on read, list and write endpoints'
description: Table-driven test over an ACL-enabled app whose handlers run without an acl.Request on ctx. Asserts 401 where a handler reaches acl.ErrNoIdentity, 500 for an identity mismatch, and that no endpoint answers 500 for a missing identity.
kind: test
location: internal/dataentry/noidentity_status_test.go
status: active
---
