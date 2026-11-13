---
id: TKT-RZSTAPZ
type: ticket
title: Refuse remote credentials outside remotes.yaml
kind: enhancement
priority: medium
effort: s
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

## Description

Two deferred security review findings from the rela-to-rela sync work
(FEAT-CMVZQFI).

### RR-Q09NYPQ: only the remote config may reference a remote credential

A remote credential is read from a `remote_credential.*` secret, an absolute
file named `remote_credential.*`, or an environment variable
`REMOTE_CREDENTIAL_*`. Every surface that names a secret already refuses
`remote_credential.*` (since #2165): subscriptions `auth.secret`,
connections.yaml `client_id_secret`/`client_secret_secret`, flows webhook
`secret`, schema and mail.yaml `capabilities.secrets`, and the Lua secret
filter. The AI and mail secret keys are fixed names (`ai_api_key`,
`smtp_password`, `mail_api_token`).

Two surfaces still name an environment variable freely: ai.yaml `api_key_env`
and mail.yaml `password_env`. Either can name `REMOTE_CREDENTIAL_UPSTREAM` and
send the remote credential to the AI base URL or the mail server.

Fix: move the env prefix to `internal/secrets` (`RemoteCredentialEnvPrefix`,
`IsRemoteCredentialEnv`), use it in httpremote, and refuse it in ai.yaml and
mail.yaml validation. One test per surface.

### RR-T2DQV3D: gate hops keep the ACL world

Assess only. See the finding's reason for the result.

## Acceptance

- ai.yaml and mail.yaml with an env name under `REMOTE_CREDENTIAL_` fail validation with a message naming remotes.yaml.
- Every surface that names a secret or env variable has a test refusing a remote credential.
- RR-Q09NYPQ addressed; RR-T2DQV3D closed with a reason.

## Notes

- Code review added: `cmdexec` withholds `REMOTE_CREDENTIAL_*` from every child process.
- CodeQL treats `api_key_env` and `password_env` as credentials, so the refusal errors name the prefix, not the configured name (RR-4BT1TIJ).
