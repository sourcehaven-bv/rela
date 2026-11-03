---
id: RR-ZP5F9EJ
type: review-response
title: Bound the re-load batch and stop at one past the cap
finding: 'Design review: resolve at most a bounded number of refs per call, walk violations in order, stop at 101 issues, and treat an unparsable violation id as a miss.'
severity: minor
resolution: resolveBatch = 512; both loops walk in order and stop at one past maxSectionIssues; entity.ParseRef failure is a miss. TestAnalyzeValidations_CapBoundary and TestAnalyzeOrphans_CapBoundary cover 100, 101, two rules and a batch boundary.
status: addressed
---
