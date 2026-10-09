---
id: RR-E18H0H
type: review-response
title: CalDAV root branch not covered by the link test
finding: TestOpenAPI_RootLinksToSpec does not set caldavAliases, so the CalDAV mount of / is covered only structurally.
severity: nit
reason: withSpecLink wraps the SPA handler before the CalDAV branch, so both mounts receive the same handler by construction; a CalDAV fixture adds setup for no extra assurance.
status: wont-fix
---
