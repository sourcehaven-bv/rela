---
id: RR-0ZQ9V55
type: review-response
title: "Console spy in the FlowRunCard 422 test guards a path that never logged"
finding: "The step form never logged its refusal, so the spy pins nothing new."
severity: nit
reason: "It pins that the step form keeps logging nothing, which is the user-visible symptom the report was about."
status: wont-fix
---
