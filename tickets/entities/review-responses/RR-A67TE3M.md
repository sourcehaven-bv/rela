---
id: RR-A67TE3M
type: review-response
title: E2E does not cover decisions in the header in a working state
finding: "The quiet room gave the owner no decision in Planning, so AC 4a was covered only by unit tests."
severity: nit
resolution: "The owner may Approve in Planning; the e2e asserts the header holds it, Go to decision is absent and the fold stays whole."
status: addressed
---
