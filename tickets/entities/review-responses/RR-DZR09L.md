---
id: RR-DZR09L
type: review-response
title: '`on.<type>: ~` (null) changes keys and reason'
finding: 'yaml.v3 does not call UnmarshalYAML for a null node; it zeroes the value. rawBranches{} then has no entries and list=false, so the plan''s empty-list refusal ("name at least one entry") fires at on.<type>. Today the zero rawTransition gives "name the state this transition moves to" at on.<type>.to. This contradicts Q8 ("problem keys unchanged for the mapping form"). Fix: treat a null value (non-list, zero entries) as one empty mapping entry, or pin the new reason in a test and state the change.'
severity: minor
resolution: A null `on.<type>` is treated as one empty mapping entry, so it keeps today's problem at `on.<type>.to`, pinned by a test.
status: addressed
---
