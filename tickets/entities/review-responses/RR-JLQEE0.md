---
id: RR-JLQEE0
type: review-response
title: E2E assertion can pass vacuously and measures once
finding: No guard that the cells are actually narrow; a single evaluateAll right after first paint; one viewport.
severity: minor
resolution: Spec asserts the span-2 cell is narrower than 120px before measuring; the measurement is polled with toPass; it runs at 1100px and 780px. Verified that all three new tests fail on the original CSS.
status: addressed
---
