---
id: RR-PDK8UPM
type: review-response
title: settle does not check that all work ran
finding: Leftover scheduled work after settle would show up as a false counterexample.
severity: minor
resolution: settle asserts that the scheduler holds nothing.
status: addressed
---
