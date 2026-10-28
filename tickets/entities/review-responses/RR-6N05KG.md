---
id: RR-6N05KG
type: review-response
title: Picker details
finding: 'Debounce the gated searchEntities call; the plan drops stale answers but sends one request per keystroke. Without a user type, map "authors seen" by form: a job-identity author must be inserted as `` `integration:x` `` (not `@integration:x`, which classifySpans refuses), and `unknown` and rela identities must be left out.'
severity: nit
resolution: Picker details carried into follow-up a (debounce; job authors as `integration:x`; unknown and rela identities left out).
status: addressed
---
