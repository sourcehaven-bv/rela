---
id: BUG-RON3P49
type: bug
title: idp-sync.lua ignores every webhook when idp_org_id is missing
description: rela.secrets leaves out a missing secret, so the empty-string check never fires and every event looks like another org's.
priority: high
why1: With idp_org_id missing, idp-sync.lua answered every webhook as ignored, so deactivations were lost.
why2: rela.secrets leaves out an unset secret, so the value was nil and never equal to the event org.
why3: The script checked only for empty strings, and no test covered a missing secret.
why4: The example script was written against an older binding that returned an empty string for an unset secret.
why5: 'Systemic: the script failed open on missing configuration, so a setup error showed up as silently ignored events instead of an error.'
prevention: The script treats nil as empty and fails closed, and a test covers each missing secret (#2250).
started: "2026-11-27"
completed: "2026-11-27"
status: done
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
