---
id: RR-CUW2W5
type: review-response
title: 'propertyElements Nil: contract documented the opposite of the code'
finding: 'The `Nil:` tagged doc comment on propertyElements claimed a nil property yields a single empty-string element. It did not — nil fell through to the default branch and rendered as the Go literal "<nil>" via fmt.Sprintf. Worse, "<nil>" is a matchable string, so filter[x][contains]=nil selected every entity whose property is explicitly null. A wrong nil-contract is worse than none: the convention exists so the next reader can trust it.'
severity: critical
resolution: 'Added an explicit `if prop == nil { return []string{""} }` guard so the code now does what the contract promised, and rewrote the comment to explain WHY (the <nil> string is matchable) rather than just what. Verified: filter[tags][contains]=nil now returns no rows, and filter[tags]= correctly matches the null-valued entity. Both pinned as test cases in TestV1FilteringListProperty_EdgeCases.'
status: addressed
---
