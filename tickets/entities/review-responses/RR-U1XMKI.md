---
id: RR-U1XMKI
type: review-response
title: rela.document.entry_id becomes an address for faced rows
finding: Scripts use entry_id as a bare id (trace_to, relation filters). Passing refOf(ent).String() changes it to ID@face for faced rows; the contract change is undocumented and unpinned.
severity: significant
resolution: 'entry_id is the cleared row''s address: bare id for faceless types (unchanged), ID@face for faced rows (which previously 500''d). Documented in docs/lua-scripting.md and docs/data-entry.md; pinned by ENTRY assertions in TestAnchoredDocument_FaceGate including the faceless feature case.'
status: addressed
---
