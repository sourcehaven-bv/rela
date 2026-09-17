---
id: RR-Q8S0VY
type: review-response
title: Pre-existing duplicate-id merge in filecomments and memcomments
finding: 'Found by the new rename-collision conformance case rather than by either reviewer. filecomments.Rename and memcomments.Rename merged into an occupied destination with a blind append, so a destination already holding a comment with the same id produced a thread containing that id TWICE (observed: [dup c1 dup]). The service addresses a comment by (target, id) and Service.Get linear-scans for the first match, so the second copy is unreachable and an Update or Delete aimed at it silently hits the other one. Predates this ticket — both backends shipped with it — and was invisible because the existing merge test used only distinct comment ids.'
severity: significant
resolution: 'Hoisted comments.MergeThreads into the shared package beside SortComments, for the same reason that one is shared: every backend owes the same answer, and two implementations agreeing independently is how that quietly stops being true. It drops an arrival whose id the destination already uses, matching the database backends'' ON CONFLICT DO NOTHING, and sorts. Both file and memory backends now call it. Also hoisted FacePrefixPattern, which had been duplicated byte-for-byte across the two database backends including its doc comment — the commentlint duplication rule''s exact target.'
status: addressed
---
