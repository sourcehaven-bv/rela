---
id: RR-8GASN1N
type: review-response
title: "A required checkbox left unticked got no message"
finding: "missingRequired counted false as a value, so an unticked Terms checkbox that the condition reads as true stayed unmarked."
severity: significant
resolution: "An unticked checkbox the step requires now counts as empty; FlowRunCard.test covers it."
status: addressed
---
