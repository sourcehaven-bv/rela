---
id: RR-4B7VVC
type: review-response
title: Abort message advised a reload for a cause a reload cannot fix
finding: 'A link_as: to prefill with a prefix-less peer id registers no type, so it reaches the abort path. The new message asserted "a field this form DOES render could not type one of its own targets — a reload may genuinely help", which is false for a relation with no field on the form: a reload changes nothing.'
severity: critical
resolution: relationTypeErrorMessage now checks whether any untyped key is owned. A rendered relation keeps the reload advice; a prefill-only key gets "This form has no field for it, so it arrived as a pre-filled link. Ask your operator to check the originating create button section config" — mirroring the existing post-condition wording. Pinned by "does not advise a reload for an untypeable unrendered prefill".
status: addressed
---

placeholder
