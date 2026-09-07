---
id: RR-P1YQPB
type: review-response
title: ConstEqualities reported an empty-string literal whose store meaning differs from the Go pass
finding: '`entity.assignee == ''''` lowered to PropPredicate{Value: ""}, which every backend reads as ''is empty'' (absent, null, empty list), while the Go pass reads Nil == String("") as false. Direction safe (store keeps a superset) but two operators of one expression disagreed on the unset population, with no test.'
severity: minor
resolution: constEqualityFrom refuses an empty literal (pushes nothing), with the reason in a comment; TestConstEqualities gained the case.
status: addressed
---
