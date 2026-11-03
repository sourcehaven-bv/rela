---
id: RR-Y9PMCXQ
type: review-response
title: "Budget ops ignore the fixture ACL pass"
finding: "The ops replaced the ACL readsFor passes in, so its two-pass design no longer guarded anything."
severity: minor
resolution: "The tests no longer go through readsFor; fieldPolicyReads measures only the field-policy shape, and its doc comment says so."
status: addressed
---
