---
id: RR-KAB5HUB
type: review-response
title: The step form sends every field
finding: FlowStepForm sent every writes value it loaded, so it could overwrite a concurrent edit and be refused for a field the user did not touch.
severity: minor
resolution: The form keeps the loaded values and sends only the fields the user changed.
status: addressed
---
