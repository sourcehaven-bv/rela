---
id: RR-EWX83N5
type: review-response
title: Hard link passes the resolved-name check
finding: A hard link named remote_credential.x to another file passes EvalSymlinks and the name check.
severity: minor
resolution: 'openDeclaredFile refuses a file with more than one hard link (unix). Test: TestCredential_ReadDeclaredHardLink.'
status: addressed
---
