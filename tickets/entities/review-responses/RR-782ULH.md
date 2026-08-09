---
id: RR-782ULH
type: review-response
title: date StringType->DateType is a semantic verdict change on the ACL boundary, not 'unchanged'
finding: 'Plan AC1/Security assert date when: predicates ''still evaluate correctly'' / behavior neutral. False for ordered comparisons: today date=StringType (affordances/env.go:110-112) so entity.due < ''...'' is a LEXICOGRAPHIC compare of raw stored strings (incl. non-zero-padded, tz-suffixed, datetime-with-time values); after swap date=DateType = instant-granular time.Time ordering (Before/After/Equal, strict-instant equality for datetime). Where the two orderings disagree, a visible:/fields: grant verdict flips = who-can-see-a-field changes. Well-formed zero-padded ISO dates agree, so happy-path tests hide it. FIX: reframe AC1 as an INTENTIONAL lexicographic->instant shift on the auth boundary; add distinguishing golden verdict tests with adversarial stored values (''2026-1-5'', ''2026-01-05T23:59:59Z'') that separate the two orderings, not just confirm the happy path.'
severity: critical
resolution: 'Reframed as intentional. date StringType->DateType is now instant-granular. TestResolver_DateWhen_SemanticChange pins the observable ACL-boundary changes: (1) an unparseable non-padded date (''2026-1-5'') now binds Nil -> fail-closed deny (desired: ill-formed dates don''t grant via lexicographic luck); (2) a datetime with time-of-day (''2026-06-01T08:00:00Z'') compares instant-granular (AFTER midnight Jun 1 -> < ''2026-06-01'' is false). For well-formed zero-padded ISO dates lexicographic and instant orderings agree, so no silent flip there. Documented as intentional in the test.'
status: addressed
---
