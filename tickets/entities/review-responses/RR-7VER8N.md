---
id: RR-7VER8N
type: review-response
title: Restore of a deleted face could overwrite a live face (ApplyEntity is an upsert)
finding: 'restoreRecreate and the CLI restore used ApplyEntity, which picks create or update from its own existence probe. A face recreated between the history resolve and the write turned the restore into a whole-record update: properties the snapshot lacks were erased (including ones the caller cannot read or write), the per-field gate had only checked sets, and no automation ran.'
severity: critical
resolution: Added entitymanager.RecreateEntity (package function plus a Recreator adapter, since Manager's method count is pinned). It shares ApplyEntity's body but refuses the update branch with ErrEntityAlreadyExists. The HTTP restore and rela restore use it; the HTTP path maps the error to 409 state_changed. Pinned by TestRecreateEntity_IsCreateOnly and TestFacedHistory_RecreateNeverOverwritesALiveFace (409 and the live row unchanged).
status: addressed
---
