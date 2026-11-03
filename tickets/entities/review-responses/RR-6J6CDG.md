---
id: RR-6J6CDG
type: review-response
title: Logo upload and delete leaf handlers carry no archplan auth=authz marker
finding: The settings:edit check for the logo sits in the method switch handleAPIThemeLogo, so the leaf handlers handleAPIPutThemeLogo and handleAPIDeleteThemeLogo carry no auth=authz annotation. (rela-security-reviewer, re-review)
severity: nit
resolution: Deferred to the archplan-markers change.
reason: The archplan markers on those leaf handlers are added by the separate archplan-markers change, which must mark them auth=authz now that their route is gated.
status: deferred
---
