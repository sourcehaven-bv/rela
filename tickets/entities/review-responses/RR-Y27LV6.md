---
id: RR-Y27LV6
type: review-response
title: AC 4 leaves two options open
finding: 'AC 4 offers two ways to test it: add default_in to the SameForEveryPrincipal fixture, or compare across principals in TestChannelSchema_In. The test-plan table picks the second. Remove the first from the AC so the implementer has no choice to make.'
severity: nit
resolution: "addressed\tAC 4 now names one test: TestChannelSchema_In compares bodies for two principals."
status: addressed
---
