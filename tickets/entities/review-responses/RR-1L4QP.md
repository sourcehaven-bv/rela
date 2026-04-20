---
id: RR-1L4QP
type: review-response
title: 'Critical: factory open path never cross-checks .rela/repo-id against keyring RepoID — rollback defense silently broken on fresh clones'
finding: 'cranky-code-reviewer #1: workspace.Discover calls userstate.Open before loading the keyring, so on a fresh clone of an encrypted repo a random .rela/repo-id is generated. last_seen_version and the reseal sentinel then get keyed under the wrong dir. rollback detection no longer protects fresh clones — the exact window the ticket was meant to close.'
severity: critical
resolution: 'Added userstate.VerifyKeyringRepoID call in app.FSFactory.loadEncryption immediately after LoadFromDir returns, before resuming any rotation. Now every factory open path (workspace.Discover, cmd/rela-desktop, mcp server, scheduler) runs the cross-check, not just keys CLI commands. See internal/app/factory.go. Also: userstate.Open writes a .rela/repo-id for new cleartext repos; on encrypted repo with missing repo-id the keyring''s RepoID is adopted via the verify path.'
status: addressed
---
