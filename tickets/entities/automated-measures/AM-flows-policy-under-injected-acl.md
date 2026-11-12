---
id: AM-flows-policy-under-injected-acl
type: automated-measure
title: 'Test: flows compile against acl.yaml under an injected non-declarative ACL'
description: Boots a project whose flows.yaml names roles with WithACL(NopACL) and asserts the boot succeeds and the flow engine is built.
kind: test
location: internal/appbuild (flowruntime_test.go, added in the BUG-E9A72XB fix PR)
status: active
---
