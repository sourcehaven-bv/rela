---
id: RR-LJKSOC1
type: review-response
title: Past reads judged relation-based field grants on past edges
finding: 'Field visible: grants held through a role relation or a related(...) condition read the edges at the instant while the row gate reads today''s edges. A reader who lost such a grant could still see the field in a past read.'
severity: significant
resolution: The as_of middleware marks the request with affordances.WithHistoricalSubject so field visibility follows the history endpoint's fail-closed rules; SearchFieldScope compiles the same rules. Pinned by TestAsOf_SQLite_RelationConditionedFieldFailsClosed and the @history cases of TestSearchFieldScope_AgreesWithFieldVerdicts (both mutation-checked).
status: addressed
---
