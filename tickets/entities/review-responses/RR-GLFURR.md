---
id: RR-GLFURR
type: review-response
title: Orphan batch restored to a different entity that reuses the id within 30d
finding: internal/realtime/retention.go:243-244 untombstones an orphan batch when its id reappears, including a new, unrelated entity created through rela. The EntityCreated hook (internal/appbuild/channels.go EntityCreated) only clears live channels, and Channel.Incarnation, which exists to fail closed on id reuse, is never set or compared. Refile pending/orphan batches as non-restorable detach in EntityCreated, or compare incarnation before Untombstone.
severity: significant
resolution: 'Same fix as the previous finding: create-time ConfirmTombstones plus the store''s refusal to restore a final batch.'
status: addressed
---
