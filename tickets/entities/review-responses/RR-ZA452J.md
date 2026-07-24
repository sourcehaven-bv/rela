---
id: RR-ZA452J
type: review-response
title: rela.md.entity_refs uses context.Background() — binding returns empty for every user on ACL paths
finding: 'markdown.go:2369 keeps a pre-existing `ctx := context.Background()` while now reading through the ACL-bound reader. ScriptReader resolves identity from ctx, so a background ctx has no principal → ForPrincipal returns ErrUnstampedPrincipal → permittedIDs drops EVERY type fail-closed. Reviewer proved it: alice, explicitly granted read:[ticket], gets refs=0 while get_entity/list_entities work. Not a leak (fail-closed) but a TOTAL functional break of the binding on every identity-bearing path, and SILENT — an empty map is a legitimate return, so doc-generation scripts emit unlinked output instead of erroring. Fix: ctx := r.callerCtx(), matching every other binding.'
severity: critical
resolution: 'Fixed: markdown.go now uses r.callerCtx() like every other binding, with a comment naming the failure mode. Verified the binding works for a granted principal instead of returning empty.'
status: addressed
---
