---
id: RR-1IBB49
type: review-response
title: Attachments are outside the entity content/hash model — silently not synced
finding: 'The plan''s manifest covers entities + relations only. Verified: attachments are a wholly separate storage concern (AttachmentManager, store.go:209-224), keyed by (entityID, property) — pgstore BYTEA table (attachment.go:29-67), fsstore files on disk (attachment.go:14-38). The entity.Entity struct (entity.go:43-50) has NO attachments field, so a content-hash over Properties+Content will NOT detect attachment changes. Worse: re-attaching a file (ON CONFLICT DO UPDATE, attachment.go:60-62) does NOT bump the entity row''s updated_at/seq — so an attachment-only change is invisible to both the hash AND a seq-based manifest. Result as planned: attachments silently never sync, and there is no signal that they changed. The plan must either (a) explicitly scope attachments OUT with a documented limitation in the ticket/acceptance criteria, or (b) add a separate attachment sync channel (manifest entry per (entityID,property) with its own hash/seq + tombstone). At minimum this must be a conscious, documented decision, not an accidental gap.'
severity: significant
resolution: 'Plan updated (Approach §6 + Scope + AC): attachments explicitly scoped OUT of this ticket as a documented limitation (they''re outside entity.Entity, and an attach doesn''t bump entity seq/updated_at so they''re invisible to hash+manifest). A follow-up ticket will add a per-(entityID,property) attachment sync channel with its own hash/seq + tombstone.'
status: addressed
---
