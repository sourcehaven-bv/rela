---
id: RR-QARLC61
type: review-response
title: Form widget errors could contradict
finding: widgetAcceptsProperty says only select can render a valued property while the new check deprecates select.
severity: minor
reason: widgetAcceptsProperty runs on view section fields only; form fields never reach it, so no form gets both messages.
status: wont-fix
---
