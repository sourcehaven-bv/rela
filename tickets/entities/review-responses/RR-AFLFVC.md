---
id: RR-AFLFVC
type: review-response
title: literalMessage could reuse NormalizeProps now that it lives in realtimecfg
finding: 'internal/realtimecfg/validate.go:909-941 still applies its own subset of the prop rules (max length, message_bytes) and builds predicate fields by hand, while NormalizeProps and PredicateFields sit in the same package. Fix (follow-up): run trigger and reminder literals through NormalizeProps and PredicateFields, so that all three load-time checks and the post share one rule set.'
severity: nit
reason: 'The reviewer marks it a follow-up: routing trigger and reminder literals through NormalizeProps changes their refusal messages and load behaviour, which is outside this ticket. It should get its own ticket.'
status: wont-fix
---
