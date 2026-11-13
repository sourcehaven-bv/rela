---
id: AM-acl-mismatch-warns
type: automated-measure
title: 'Test: a schema-mismatched acl.yaml under an injected ACL boots with a warning'
description: The subtest "acl.yaml that does not fit the schema still boots" asserts that loadConfigPolicy logs the validation error as a warning instead of discarding it.
kind: test
location: internal/appbuild/flowruntime_test.go (TestFlowEngine_InjectedACLConfigPolicyEdges)
status: active
---
