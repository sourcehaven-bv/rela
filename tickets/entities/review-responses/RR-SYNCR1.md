---
id: RR-SYNCR1
type: review-response
title: 'Sync client relation DTO must move to the /api/v1 relation shape; needs a v1 relation read carrying body + usable ETag'
finding: 'The fancy-browser model routes pull-fetch through /api/v1. The current /api/v1 relation read (handleV1EntityRelations / buildRelationTypeRows, api_v1.go) returns rows keyed to a source entity as {id: peerID, type, meta: edge.Properties} — it carries NO relation Content (body), no flat from/type/to triple, and no per-relation canonical hash (the ETag on that response is the entity''s computeEntityETag). The sync client''s GetRelation decodes a flat whole-record RelationBody + reads the ETag header as the relation hash. So the current /api/v1 relation surface does not serve what the sync client needs. NOTE: this is NOT "the SPA cannot read relations" (it can) — it is that the sync client''s relation DTO must move to the v1 shape, and the v1 relation read must expose the relation body + a usable ETag for pull. Relation versioning (TKT-92JL8P) is explicit that relations carry their own props + body, so dropping the body on the read path is a real gap for a faithful replica.'
severity: significant
status: addressed
resolution: 'Added handleV1GetRelationTarget (internal/dataentry/relation_read_handler.go), wired as the GET case of handleV1RelationTarget. It returns {from,type,to,meta,content,_redacted} + a relation-level ETag over the RAW relation (canonical.HashRelation, reader-independent). Dual-endpoint gated (both FROM and TO readable on their live types, indistinguishable 404 otherwise); meta redacted via visibleRelationMeta and FAIL-CLOSED (no meta when the source is not live). The sync client GetRelation decodes this shape; the client relation DTO moved to it.'
---

## Finding (design-review, fancy-browser)

`/api/v1`''s relation read shape (`{id, type, meta}` keyed to a source entity)
does not match the sync client''s relation DTO (flat `from/type/to` + `content`
body + per-relation canonical-hash ETag). Retiring `handleSyncGet` for relations
therefore needs one of:

1. a `/api/v1` single-relation read that returns the relation **body** and a
   **relation-level ETag** the replica can use as its baseline, gated by the same
   read ACL (both endpoints FROM ∧ TO for relations, per the relation-history
   live-world rule); and
2. the sync client''s relation DTO + `GetRelation` rewritten to that v1 shape.

This is client-rewrite work, not a blocker to the model — but it must be planned,
because "read via /api/v1" silently assumed the entity story generalizes to
relations, and for the body + hash it does not yet.

## Recommended resolution

Add/confirm a v1 relation read that carries body + relation ETag under the
existing relation read gate; move the sync client relation DTO to it. Keep the
row-gate parity (a relation in the feed is fetchable). Field-level meta redaction
is inherited from the v1 read path (fail-closed on dead source, per the existing
relation-history pattern).
