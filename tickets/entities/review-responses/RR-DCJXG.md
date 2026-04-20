---
id: RR-DCJXG
type: review-response
title: formatter.go bonus-fix framing undersells an active correctness bug
finding: 'Plan classifies formatter.go:31,67 as ''latent bug'' that ''fixes itself.'' That bug is active on encryption-whole-repo branch RIGHT NOW: every ''rela format'' on encrypted repo rewrites every entity file. Demoting to footnote risks being forgotten if refactor scope is reduced mid-flight.'
severity: minor
resolution: Plan promotes formatter.go bug from 'latent / fixes itself' footnote to explicit scope item. Migration step 2 commits a failing regression test as t.Skip('blocked on TKT-8S1SA'), unskipped as last step. AC#12 gates completion on test passing.
status: addressed
---
