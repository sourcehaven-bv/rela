---
id: rename-acl-fail-closed-test
type: automated-measure
title: 'Test: RenameEntity fails closed when ACL pre-fetch errors'
description: 'Regression for BUG-RIM0CT: a flakyGetStore returns a non-not-found error from GetEntity under a deny-all ACL; the test asserts RenameEntity surfaces the error and performs zero store writes (no ACL bypass). A companion test pins that a genuine not-found still returns ErrEntityNotFound. Fails if the rename pre-fetch reverts to skipping ACL on any non-nil error.'
kind: test
location: internal/entitymanager/rename_acl_test.go (TestRename_FailsClosedOnNonNotFoundFetchError, TestRename_NotFoundStillReturnsTypedError)
status: active
---
