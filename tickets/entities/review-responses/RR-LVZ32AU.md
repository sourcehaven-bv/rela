---
id: RR-LVZ32AU
type: review-response
title: Credential.Read regressed for --token-file pipes
finding: readCredentialFile resolved symlinks and required a regular file for every caller, breaking --token-file /dev/stdin.
severity: minor
resolution: Only ReadDeclared uses openDeclaredFile; Read opens the path as before.
status: addressed
---
