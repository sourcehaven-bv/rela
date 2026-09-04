---
id: RR-R6G2VM
type: review-response
title: 'Relation identity must widen on the fs side: keys and filenames carry the tail face'
finding: 'The plan covered pg''s relations-PK widening (decision 3) but missed the fs equivalent: relation KEY and FILENAME are `from--type--to` (fsstore/relation.go:17, delete path entity.go:345), so two edges on the same triple differing only in FromFace would collide on one file/key — the fs representation could not store both. The memstore map key has the same shape.'
severity: significant
resolution: 'Plan updated (PR-A work list): the FROM slot serializes via the codec when FromFace is non-zero — key/file `PAGE-1@draft--implements--SPEC-9(.md)` — in fsstore and memstore; storetest case pins same-triple-two-tails stored, matched, and cascaded independently. Depends on RR grammar fix (no `--` inside a pointer).'
status: addressed
---
