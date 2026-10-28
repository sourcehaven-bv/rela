---
id: RR-IYDFHJ
type: review-response
title: 'Minor plan gaps: placement rule, filename vs format, hash read-back race, sqlite batching, tx wrappers'
finding: Placement differs from rela.tag_version; filename extension can disagree with the transform; reading the hash back can race a replace; sqlite backfill batching wording; memstore/fsstore tx wrappers and attachment.Info need SHA256; capability gate; bypass_acl interaction; shared transform pool.
severity: minor
resolution: 'Plan: placement stated precisely; extension appended from the transform output when missing and a mismatching extension is refused; attachment.Service hashes the bytes it hands to the store and returns the hash in its result so there is no read-back; sqlite step reads rowids then updates in one step; tx wrappers listed; no new capability (calling a write binding is the author''s choice and transforms are operator-registered); attach_render not on the elevated handle with a test under bypass_acl; shared pool documented.'
status: addressed
---
