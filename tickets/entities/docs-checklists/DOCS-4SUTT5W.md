---
id: DOCS-4SUTT5W
type: docs-checklist
title: 'Docs: Declarative peer pull: peers.yaml apply side'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the ledger states, held rows and delete window (ledger.go), the gone guard and emptied check, the three-way merge, relation meta validation and the dry-run byID tracking (run.go), the run lock and stale retry (scheduler/peers.go)
- [x] Function/type docs if public API: peersync.Config, Peer, Mapping, Deps, Options, Report, Counts, Issue, Run, Dial, Parse and Compile; scheduler.LoadPeers, AddPeers, PullPeer, ErrPeers, ErrPeerBusy

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README docs table is generated from guide entities; docs/peer-sync.md becomes a docs-project guide in TKT-0FMYPEZ, whose acceptance asks for generated guides)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: the pull follows existing patterns: entitymanager writes, state.KV, the scheduler run lock)
- [x] Help text accurate (if CLI changes): `rela peer pull` help, the `rela peer` group help, and the `rela scheduler` no-tasks message name peers.yaml

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable): docs/peer-sync.md (configuration, merge rules, gone rows, pull by hand, state, security); GUIDE-acl-security "Peer pulls (`peers.yaml`)" threat model; GUIDE-cli-reference `rela peer pull` and the reserved `system:peer:` prefix; GUIDE-scheduled-tasks peers.yaml tasks; #2157 and #2158 fixes
