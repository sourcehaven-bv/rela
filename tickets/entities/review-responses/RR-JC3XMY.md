---
id: RR-JC3XMY
type: review-response
title: DTEND contract must be uniform (verbatim/exclusive) across all-day and timed
finding: 'calfeed.go:49 documents End as the ''exclusive'' end day, but feed_provider.go:232-239 sets ev.End = end VERBATIM (no +1 adjustment) — so exclusivity is actually the author''s responsibility today. The timed branch must NOT introduce a different meaning: End renders verbatim for both types (only the FORMAT differs, DATE vs DATE-TIME). Risk: an author sets end_date==date for an all-day event thinking ''inclusive'', producing DTEND==DTSTART which RFC5545 3.8.2.2 treats as zero-duration/invalid for VALUE=DATE. Fix: keep End = the RFC5545 DTEND value as-rendered (verbatim) for both; the RenderEvent branch changes only format, never End''s meaning. Add tests: all-day range renders the author''s end verbatim; timed range renders the exact end instant.'
severity: critical
resolution: 'Adopted verbatim/exclusive contract, uniform across both types. No coercion, no +1 adjustment anywhere — the RenderEvent branch changes only the format (VALUE=DATE vs timed Z). Update the Event.End doc comment to state ''rendered verbatim as the RFC5545 DTEND value (exclusive)'' for both. Tests pin: all-day range DTEND == author value; timed range DTEND == exact instant.'
status: addressed
---
