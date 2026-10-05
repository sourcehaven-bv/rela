---
id: RR-45DZM2
type: review-response
title: No pass-through test for the world in CLI list and scheduled for_each/mail
finding: Only mailtemplate.Build has a test that the supplied world reaches the query. A dropped World field in the other three sites is invisible while Default() returns the zero value.
severity: minor
reason: No observable difference exists until TKT-7IZHP0 changes Default(). PR 8 adds the EntityQuery literal guard and the DefaultWorld() guard, which catch a missing selection at test time; Stage 3 adds behavioural tests when the default world stops being the zero value.
status: deferred
---
