---
id: RR-E2GQGQV
type: review-response
title: No test pins field redaction on the context path
finding: '[security] contextCandidates redacts the viewed entity before the message is built, but no test pins it. A regression would leak visible:-hidden values into the suggestion message.'
severity: minor
resolution: Added TestNextAction_ContextRedactsHiddenProperties.
status: addressed
---
