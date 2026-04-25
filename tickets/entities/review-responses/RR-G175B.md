---
id: RR-G175B
type: review-response
title: Add 'load every shipped metamodel' regression test
finding: TKT introduces a new field nobody is using yet. Day someone adopts it in tickets/metamodel.yaml or docs-project metamodel and typos the property name, failure surfaces only when somebody runs that specific metamodel.
severity: minor
resolution: Add (or confirm) a test that loads every metamodel.yaml under the repo (tickets/, docs-project/, prototypes/) via metamodel.Load and asserts no error. Five lines of test code; cheap insurance for the day someone adopts display_property in a dogfood metamodel.
status: addressed
---
