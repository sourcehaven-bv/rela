---
id: RR-QPQAADH
type: review-response
title: Relative credential file resolves against the working directory
finding: A relative file path in remotes.yaml would read a different file under rela-server or the scheduler than under the CLI.
severity: minor
resolution: A credential file must be an absolute path (httpremote.Credential.ValidateDeclared); config test relative file.
status: addressed
---
