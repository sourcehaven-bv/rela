---
id: RR-8UQI0W
type: review-response
title: 'Code: tenant opener will default to foreground'
finding: tenant.AppBuildOpener calls appbuild.New without options.
severity: minor
reason: The tenant opener is not wired anywhere yet; options belong with that wiring.
status: deferred
---
