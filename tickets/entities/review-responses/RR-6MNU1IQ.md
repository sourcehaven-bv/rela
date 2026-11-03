---
id: RR-6MNU1IQ
type: review-response
title: Configure save refused while a profile is set
finding: With RELA_AFFORDANCE_PROFILE=none or demo set on a running rela-server a Configure save that adds the first field rule to acl.yaml now fails to build the new generation. The plan did not say whether that is intended or how the operator learns why.
severity: minor
resolution: 'Intended: the save fails like any other generation build error and the old generation keeps serving. The error names the variable and the cause. Documented in the bug body.'
status: addressed
---
