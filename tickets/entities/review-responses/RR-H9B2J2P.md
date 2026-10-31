---
id: RR-H9B2J2P
type: review-response
title: Property test server is too kind
finding: The test server never fails or aborts writes, has no prepare step and never follows holds with a real write, so those rules go unexplored.
severity: minor
resolution: The property test server now fails writes before or after they reach it, has reads that wait in prepare while writes complete, and turns held edits into real writes.
status: addressed
---
