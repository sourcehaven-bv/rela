---
id: RR-00RYMD
type: review-response
title: A fold anchor resolves to the run's last member when the fold dissolves
finding: 'The fold anchor is the run''s last seq (channelAnchor.ts:27, data-fold-seq). When Show all is pressed, or a config reload drops display, the fold is gone and findAnchor (channelAnchor.ts:33-40) falls back to rowFor(rows, last). That is the run''s last member, so it takes the button''s offset and the rest of the run, which the reader was about to see, opens above the viewport. AC 7 wants the first visible row to keep its place: the line the reader was looking at should become the run''s first message. Fix: store first and last in Anchor. Look up a fold directly by last, as now, so appends keep working. When the fold is gone, use rowFor(rows, first). Extend ''Quiet types, scrolled up'' so the first visible row is a fold line.'
severity: minor
resolution: Fixed. Anchor carries first; the fold li carries data-fold-first. findAnchor looks up a fold by its last seq as before and, when the fold is gone, falls back to rowFor(first). "Quiet types, scrolled up" now scrolls so a fold line is the first visible row and checks the run's first message takes its offset after Show all; it fails with the old fallback.
status: addressed
---
