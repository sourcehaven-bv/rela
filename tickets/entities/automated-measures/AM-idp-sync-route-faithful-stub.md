---
id: AM-idp-sync-route-faithful-stub
type: automated-measure
title: Route-faithful Pratique stub for the idp-sync reference action
description: Runs the shipped examples/idp-sync.lua against a stub that serves only Pratique's real operator routes (member list GET and single-member DELETE) and verifies the HMAC signature. A table test asserts that every non-active status, a missing status, an absent subject and another org leave no roles granted.
kind: test
location: internal/lua/idpsync_test.go (TestIdpSyncAction_ClearsRolesUnlessActive, TestIdpSyncAction_IgnoresOtherOrg)
status: active
---
