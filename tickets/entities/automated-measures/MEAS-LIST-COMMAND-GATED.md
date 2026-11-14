---
id: MEAS-LIST-COMMAND-GATED
type: automated-measure
title: List-context command input is ACL-gated
description: Test that a list-context command's stdin carries only rows the principal may read, with hidden fields redacted.
kind: test
location: internal/dataentry/commands_acl_test.go
status: active
---

A test runs a list-context command as a principal whose ACL hides some rows and
redacts a field, and asserts the command's stdin carries only the visible rows
with the field redacted.
