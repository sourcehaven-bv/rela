---
id: RR-CTX5
type: review-response
title: Denied-write audit row named one arbitrary entity
severity: significant
status: addressed
finding: The dedup by (relType, fromType) is correct for the decision, but the audit row still carried
  one arbitrary FromID while representing a whole class. A forensic query for another source in the same
  class would find nothing, though it was equally refused.
resolution: FromID is omitted for the cascade check, so the row represents the type-pair, which is what
  the dedup actually decided.
---
