---
id: RR-MU7VR0
type: review-response
title: Frame arrival within a grid tick still depends on how many inbox replays hidden changes selected
finding: 'internal/dataentry/live.go:766 commits and frames only after every replay of the round finishes (wg.Wait at :760). A durable write in a hidden channel of a waiting type selects every inbox subscription whose wake set holds that type (live_inbox.go:193), so the grid-tick round runs many more replays and every visible frame on that tick arrives later; on the file channel store a full inbox round is sized to take up to one 4 s grid period. Attack path: Alice triggers a visible change of her own every grid tick (a mark read) and measures the delay from tick to frame; a jump correlates with a hidden waiting-type write. The grid fixes when a round starts, not when its frames leave, so the docs line "when a frame arrives does not depend on changes you cannot see" (docs/realtime.md, Live updates) overstates it. Same class as the existing full round a hidden entity edit triggers, hence minor. Fix: soften the doc claim to the round start, or commit inbox results in a separate pass after a fixed deadline within the grid period.'
severity: minor
resolution: The package doc, docs/realtime.md and docs/acl-security.md now say the grid fixes when a round starts, not when its frames leave; guides regenerated.
status: addressed
---
