---
id: RR-OK2KRM
type: review-response
title: Update Service.Post's check-order comment
finding: The doc comment on Service.Post (service.go:359-363) lists the checks in order. Add the session-state check after "an open channel" so the comment matches the code.
severity: nit
resolution: The approach now includes updating the check-order comment on `Service.Post` to add the session-state check after "an open channel".
status: addressed
---
