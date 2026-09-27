---
id: RR-35CXMY
type: review-response
title: Per-row ACL traversal cost on list paths
finding: 'Field/relation verdicts run per row on list serialization; caldav; feeds; tracer; PolicyReader and gantt; an unbatched when: traversal is one query per row.'
severity: significant
resolution: Resolver gets PrimeTraversals(ctx; rows) returning a ctx carrying a (type;id;spec) memo; primed at the data-entry page serialization seam and in PolicyReader chunks. Remaining per-row callers (caldav; feeds; tracer; gantt) filed as a follow-up with a baseline Counting test.
status: addressed
---
