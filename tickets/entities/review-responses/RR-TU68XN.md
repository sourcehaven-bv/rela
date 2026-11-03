---
id: RR-TU68XN
type: review-response
title: Nil-ACL variant pins dead code
finding: NewApp refuses nil, so 17 recordings (7 panics) pin unreachable arms.
severity: significant
resolution: Nil-ACL variant removed; TestNewApp_RejectsNilACL pins that NewApp refuses nil.
status: addressed
---
