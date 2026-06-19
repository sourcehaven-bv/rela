---
id: RR-ZHUHJ0
type: review-response
title: ID-prefix hard gate vs ID-preservation purpose was undocumented/untested
finding: ApplyEntity validates via ValidateEntity which includes the metamodel ID-prefix check, a HARD error (IsSoft whitelists only Required/InvalidType/InvalidValue). So a peer ID like REQ-0007 is rejected if this deployment declares a different prefix — undercutting the stated purpose (preserve a caller-supplied id). Every test ID happened to match the test prefix, hiding the gap.
severity: significant
resolution: 'Made it an explicit DECISION + test: sync assumes peers share a metamodel (so a peer-minted id matches a local prefix). A foreign-prefixed id is intentionally rejected as a hard error. Documented in the ApplyEntity godoc (''ID-prefix note'') and pinned by TestApplyEntity_RejectsForeignIDPrefix (FOREIGN-1 on a REQ- type → *ValidationError).'
status: addressed
---
