---
id: AM-remote-deleted-delete-only
type: automated-measure
title: 'Test: remote_deleted resolve needs only the delete right'
description: TestRemotes_RemoteDeletedNeedsDelete asserts that taking the remote side of an entity remote_deleted entry is offered with delete and without update, and refused with update and without delete.
kind: test
location: internal/dataentry/remotes_handler_test.go
status: active
---
