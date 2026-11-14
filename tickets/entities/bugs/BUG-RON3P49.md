---
id: BUG-RON3P49
type: bug
title: idp-sync.lua ignores every webhook when idp_org_id is missing
description: rela.secrets leaves out a missing secret, so the empty-string check never fires and every event looks like another org's.
priority: high
status: backlog
---

## Summary

`examples/idp-sync.lua` checks `base == "" or key == "" or want_org == ""`.
`rela.secrets` leaves out a secret that is not declared or configured, so the
value is `nil`. With `idp_org_id` missing, every event fails `org ~= want_org`
and the script answers 200 "ignored". The IdP does not retry, so deactivations
are lost.

## Fix

Treat `nil` as empty for the secrets and params. A test runs the shipped script
with each secret missing and expects an error result.

GitHub: #2137
