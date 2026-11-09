---
id: RR-ZMNFU5T
type: review-response
title: credential env and file could send any host secret to the remote URL
finding: Only the secret source required the remote_credential. prefix. env and file accepted any name, so whoever edits remotes.yaml could send RELA_DATABASE_URL or the token key to a URL they choose.
severity: significant
resolution: 'httpremote.Credential.ValidateDeclared: env must start with REMOTE_CREDENTIAL_; file must be an absolute path whose base name starts with remote_credential. Config tests cover both refusals and the accepted forms. Docs and the threat model state the rule.'
status: addressed
---
