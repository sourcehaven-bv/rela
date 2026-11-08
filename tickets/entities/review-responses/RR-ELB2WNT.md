---
id: RR-ELB2WNT
type: review-response
title: Engine role injection must be idempotent and normalized
finding: ValidateAgainstMetamodel runs after Validate normalized the policy and may run again on the same policy.
severity: significant
resolution: 'Plan changed: the role is built in normalized form and injection is idempotent; an operator role or assignment with the reserved names is refused earlier by Validate.'
status: addressed
---
