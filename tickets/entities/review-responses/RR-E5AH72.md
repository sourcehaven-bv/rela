---
id: RR-E5AH72
type: review-response
title: Versioning consuming rela_seq breaks the change-feed watermark overlap budget
finding: 'The plan reuses nextval(''rela_seq'') for entity_versions.seq. The multi-writer change feed''s watermark assumes every rela_seq value is consumed INSIDE a store write tx and lands in entities/relations/deletions — the exact tables primeWatermark/catchUp scan (listener.go:220-251). primeWatermark sets the watermark to max(seq)-100 (overlap budget, listener.go:33). Every rela_seq value versioning burns that does NOT land in a scanned table consumes that budget; a concurrent burst can push a genuine late-committing entity/relation seq >100 below max, so its EventEntityUpdated is NEVER emitted — a real SSE/sync change-feed data-loss path. Adding entity_versions to the watermark set is also wrong (it would emit spurious events; cf. the attachments warning at listener.go:216-219). Fix: use a SEPARATE dedicated sequence (version_seq) that the watermark never touches; never rela_seq.'
severity: significant
resolution: 'Fixed: entity_versions ordering uses a dedicated new sequence version_seq, NOT rela_seq. The change-feed watermark (primeWatermark/catchUp, listener.go:220-251) is untouched, so its max(seq)-100 overlap budget is not consumed by version writes. entity_versions is deliberately NOT added to the watermark table set. Documented in postgres CLAUDE.md.'
status: addressed
---
