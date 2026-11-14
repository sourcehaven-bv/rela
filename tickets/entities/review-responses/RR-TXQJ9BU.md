---
id: RR-TXQJ9BU
type: review-response
title: Sandboxed commands can read credential files
finding: The cmdexec sandbox mounts / read-only, so a transform or scanner can read .rela/secrets.yaml or a remote_credential.* file.
severity: minor
reason: Needs write access to operator config that Configure locks; that author already runs code. Documented in GUIDE-acl-security. A tmpfs overlay of .rela and CREDENTIALS_DIRECTORY in the sandbox is a separate change.
status: deferred
---
