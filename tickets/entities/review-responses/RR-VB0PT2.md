---
id: RR-VB0PT2
type: review-response
title: Effort s is realistic only if the AC 5 test moves to realtimecfg
finding: There are about 60 lines of config code and two one-line runtime edits. But the test plan has eight new test functions, a refactor of the affordance table, a trigger harness case, a handler case and a docs example test. That is the top end of s. Moving AC 5 to the existing realtimecfg helper (see the equivalence finding) keeps it in s. Otherwise record m.
severity: nit
resolution: AC 5 now reuses the existing realtimecfg helper, so the effort stays s, at the top end of it, as the Risk Assessment records.
status: addressed
---
