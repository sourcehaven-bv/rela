---
id: RR-HA7O28
type: review-response
title: Row-ceiling oracle is acceptable but must be documented and narrowed, and the error code should be distinct
finding: 'Judgement: counting before the gate leaks one bit at a 20 000 threshold. The only hidden rows that can contribute are channels behind a `read:` role on readable anchors, channels on faces of a readable id that the caller cannot read (ListWaiting matches any face), and channels created by the same user name in another org (created_by_user has no org). It never reveals entity existence, so it is acceptable as an accepted channel like MentionedBy.Truncated. The same bit can also end a watch on a hidden write (the answer flips to an error), which breaks the tools_watch invariant "hidden writes cannot end a wait". Fix: record it in docs/acl-security.md as accepted instead of "no realistic project reaches it". Narrow the pre-gate set: pass (id, face) pairs for face pairs, add CreatorTypes, and filter the org in Go before counting where possible. A generic 500 hides nothing, because the failure itself is the bit, but it makes the fault undiagnosable. Use a stable documented code (503 inbox_too_large). Note that an insider who may create channels can trigger it (200 per anchor per type).'
severity: minor
resolution: 'Kept as an accepted one-bit channel: 503 inbox_too_large, recorded in docs/acl-security.md with the hidden-row sources, the watch-ending exception and insider trigger. Narrowed by exact (id, face) anchors and CreatorTypes.'
status: addressed
---
