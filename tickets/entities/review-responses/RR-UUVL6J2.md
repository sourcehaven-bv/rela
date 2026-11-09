---
id: RR-UUVL6J2
type: review-response
title: Two definitions of a remote credential name
finding: isRemoteCredentialName in cli and ScriptReadable in secrets could drift.
severity: minor
resolution: secrets.IsRemoteCredential is the one predicate; the CLI uses it, and TestIsRemoteCredential asserts every accepted name is not script readable.
status: addressed
---
