---
id: RR-FC2C
type: review-response
title: 'Round 2 #5: 100-row soft cap needs a behavioural test, not just a perf smoke'
finding: |
  AC 10's "100-row mounts under 200ms" is a perf assertion, not a cap-behaviour assertion. The cap is documented intent with no executable guarantee. A separate test is needed: `section.entities.length > 100` ⇒ all rows render in display mode, zero SectionEditForm instances.
severity: minor
status: addressed
resolution: |
  PLAN AC 10 amended: add a dedicated "cap-behaviour" test alongside the perf smoke. Mounts a fixture with 101 rows; assert `wrapper.findAllComponents(SectionEditForm).length === 0`. The perf smoke (100-row mount under 200ms) tests the cap's intended capacity.
---
