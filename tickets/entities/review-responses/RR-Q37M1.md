---
id: RR-Q37M1
type: review-response
title: Handlers call a.State()/a.Meta()/etc. multiple times instead of capturing once
finding: App struct doc says 'handlers call a.State() once at entry and work against a coherent snapshot for the duration of the request'. Nobody does this. handleGraphData calls a.Graph()/a.Meta() many times; handleAPIGetSettings interleaves a.State().UserDefaults/a.Meta()/a.Graph()/a.State().UserPalette; V1Config response calls a.Cfg() five times. Each call is an independent Load. A reload between two calls produces a torn request.
severity: significant
reason: Mass migration of every handler to the snapshot-prologue pattern is a significant follow-up touching ~13 files. The doc comment is aspirational; the refactor establishes the primitive (a.State()) and the pattern but does not enforce it everywhere. Tracked as a follow-up cleanup ticket.
status: deferred
---
