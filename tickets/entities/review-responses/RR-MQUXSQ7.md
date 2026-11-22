---
id: RR-MQUXSQ7
type: review-response
title: ConditionSuperset lacks the stored-row warning
finding: '[security] ConditionSuperset matches stored rows; an ACL-gated caller judging redacted rows could disclose hidden values through row presence. LowerCondition documents this; ConditionSuperset did not.'
severity: nit
resolution: Copied the stored-row warning to ConditionSuperset and noted on PropCond that it does not check field visibility.
status: addressed
---
