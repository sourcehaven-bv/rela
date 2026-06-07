---
id: RR-MILH
type: review-response
title: aliceCtx() global, parameterless helper used cross-file
finding: 'Hardcodes User=''alice'', Tool=ToolDataEntry. Tests that want bob (TestACLGet_ETagSuppressedOnDeny) build inline — split styles. aliceCtx lives in acl_get_test.go but is implicitly used by acl_write_test.go; moving either file breaks compilation invisibly. Fix: principalCtx(user string) helper in test_helpers_test.go; every test uses it.'
severity: nit
resolution: Added principalCtx(user) helper; aliceCtx() retained as a back-compat alias.
status: addressed
---
