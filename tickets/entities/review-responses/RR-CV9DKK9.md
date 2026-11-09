---
id: RR-CV9DKK9
type: review-response
title: Lua guide does not mention remote_credential
finding: GUIDE-lua-scripting says the operator shell grants everything and names only token_key as an exception.
severity: minor
resolution: Secrets section and the operator-shell bullet now name token_key and remote_credential.* as never readable by a script.
status: addressed
---
