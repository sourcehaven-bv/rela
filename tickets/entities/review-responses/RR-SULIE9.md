---
id: RR-SULIE9
type: review-response
title: 'Review significants/minors: verdict-guard triplication, swallowed errors, unordered iteration, verdict-map conflation, tri-state dispatch'
finding: 'Bundle: (#4) the zero-ReadQueryResult defensive arm existed only in scopedSortedEntities while two new copies of the verdict switch lacked it; (#5) GraphQuery/ListRelations iterator errors returned bare 500s with the cause discarded; (#6) the subtree verdict loop iterated the sources map unordered against the file''s own determinism convention; (#7) the verdicts map conflated denied with not-a-source-type; (#9) the (forest, handled, err) tri-state encoded one decision in two variables.'
severity: significant
resolution: (#4) one shared ganttReadVerdict helper carries the fail-loud zero-value arm for all three sites; (#5) slog.Error with type/root/cause at every dropped-error site; (#6) ganttSortedKeys everywhere; (#7) explicit g.Sources membership check ahead of the verdict lookup; (#9) nil-forest-means-declined convention, single check at the dispatch point. gocognit-driven decomposition (ganttSubtreeVerdicts, collectGanttRound, ganttEdgesForType) keeps each piece under the cap.
status: addressed
---
