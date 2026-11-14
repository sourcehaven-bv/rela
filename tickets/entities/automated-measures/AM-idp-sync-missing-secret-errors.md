---
id: AM-idp-sync-missing-secret-errors
type: automated-measure
title: idp-sync errors when a secret is missing
description: Runs the shipped examples/idp-sync.lua with each of its three secrets left out and asserts an error result without asserted_roles, so the IdP retries instead of the event being ignored.
kind: test
location: internal/lua/idpsync_test.go (TestIdpSyncAction_MissingSecretIsError)
status: active
---
