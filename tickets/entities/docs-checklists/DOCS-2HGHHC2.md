---
id: DOCS-2HGHHC2
type: docs-checklist
title: 'Docs: TKT-TB2B8N5'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why `--token-secret` reads only a `remote_credential.` name, why `filterSecrets` filters through `ScriptReadable` under `AllSecrets`
- [x] Function/type docs if public API: `secrets.RemoteCredentialPrefix`, `IsRemoteCredential`, `ScriptReadable`, `Global`; `reconcileServices.Secret` keeps its `Nil:` contract; `httpremote.Authenticator` no longer points at peer identity

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: removal; the "resolve identity once, at the request edge" rule already describes the result)
- [x] Help text accurate (if CLI changes): `rela peer` is gone; `rela reconcile diff --help` describes `--token-secret` and the xor group

## User-facing Documentation

- [x] `GUIDE-acl-security.md`: "Peer keys" section removed; the Reconcile section says a sync client is an ordinary principal, the gate keys on the resolved principal, and header-mode callers without identity share one key as intended; the #2140 `source.pointer` fix kept
- [x] `GUIDE-server-security.md`: "Peer keys" section and the `peer:` reservation removed; new "Sync clients and other machine clients" section
- [x] `GUIDE-cli-reference.md`: `rela peer key create` removed; `--token-secret` describes the `remote_credential.` rule and both secret sources
- [x] `GUIDE-postgres-backend.md`: tenant-mode peer key note removed
- [x] `GUIDE-lua-scripting.md`: `token_key` and `remote_credential.*` are never given to a script
- [x] `docs/` regenerated with `just docs`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no endpoint changes; the removed 401/403 peer answers were never released)
