---
id: DOCS-K7Y67GE
type: docs-checklist
title: 'Docs: TKT-H91N91L'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why a route allowlist and not a method rule, why a bad key never falls through, why the lookup skips peers, why a stale reload fails closed, why no peer key works without acl.yaml
- [x] Function/type docs if public API: package doc and godoc for `internal/peerkey` (`Keyring`, `Source`, `ErrStale`), `dataentry.PeerKeys` and `SetPeerKeys`, `principal.Peer`, `secrets.PeerKeyPrefix`, `PeerCredentialPrefix`, `Global`; `Nil:` contracts on the new constructors and setters

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: peer keys follow the existing "resolve identity once, at the request edge" rule; no new convention)
- [x] Help text accurate (if CLI changes): `rela peer key create --help` and `rela reconcile diff --help` (`--token-secret`, the xor group)

## User-facing Documentation

- [x] `GUIDE-server-security.md`: new "Peer keys" section (create, store, grant, revoke, systemd credential, proxy bypass rule limited to `/api/v1/`, logs); `peer:` added to the reserved identities with an upgrade note
- [x] `GUIDE-acl-security.md`: new "Peer keys" section; Reconcile section keys the gate on the peer principal; issue #2140 fix (`errors[].source.pointer` = `/ranges/<index>`)
- [x] `GUIDE-cli-reference.md`: `rela peer key create`; `--token-secret` on `rela reconcile diff`
- [x] `docs/` regenerated with `just docs`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no new endpoint; the 401 and 403 `peer_route_refused` answers are documented in the guides)
