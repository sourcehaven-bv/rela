---
id: RR-YHRUQP
type: review-response
title: Layout guarded only by source regexes
finding: The DashboardView layout test matched CSS text. It broke on harmless reformatting and missed real regressions (a track or gap change without the 580px threshold; a grid shorthand with dense). No automated test checked overflow or tile spans.
severity: significant
resolution: 'Added e2e/tests/dashboard-card-width.spec.ts: at 1440, 900, 700 and 375px it asserts no grid overflow and that normal, wide and full tiles measure one track, two tracks (or the row) and the row. Mutation-checked: span 4 instead of span 2 fails 1440 and 700. The unit test keeps only the no-dense guard, widened to the grid shorthand.'
status: addressed
---
