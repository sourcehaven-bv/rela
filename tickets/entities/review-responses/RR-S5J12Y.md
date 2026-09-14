---
id: RR-S5J12Y
type: review-response
title: loadCandidates discarded meta.has_more while fixing a silent-truncation bug
finding: 'has_more on a merged all-pages response means listAllEntities hit its 50-page cap, so the candidate set is knowingly incomplete and BUG-HOB9BR is live again past that boundary. loadCandidates discarded result.meta entirely. KanbanView sets the precedent with a visible truncation banner, its comment reading "silent truncation is exactly this view''s bug class" -- which applies verbatim here. The failure is worse than silent: the user-facing message is "reload the form and try again", advice that cannot work, since a reload refetches the same 50 pages.'
severity: significant
resolution: loadCandidates now checks result.meta.has_more per target type and emits a console.warn naming the target type and BUG-HOB9BR. Chose a warning over a banner because the picker has no truncation UI and at >5,000 entities of one type the dropdown is unusable for other reasons; the goal is making the failure diagnosable rather than a lie.
status: addressed
---
