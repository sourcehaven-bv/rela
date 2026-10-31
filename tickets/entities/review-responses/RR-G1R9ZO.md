---
id: RR-G1R9ZO
type: review-response
title: AC 5 claims each problem is reported once, but assertProblem cannot verify that
finding: assertProblem (load_test.go:527) passes when any problem matches the key and the substring. It does not detect a second problem at the same key. The kind cases in AC 5 then cannot show that NormalizeProps was skipped. Add a helper that counts the problems at a key, or assert len(cfgErr.Problems) for those cases.
severity: minor
resolution: "addressed\tNew helper assertOnlyProblemAt requires exactly one problem at the key; the AC 5 kind cases use it."
status: addressed
---
