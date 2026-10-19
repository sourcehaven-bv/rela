---
id: RR-3PI99H
type: review-response
title: Promote action paths are advertised in OpenAPI when NoPromoter is wired
finding: 'internal/openapi/channels.go:181-232 emits paths for promoting actions in a process that always answers action_not_offered. Fix: note it in the description or pass a canPromote flag.'
severity: nit
resolution: kept the paths since realtime.yaml declares them and config is not secret; the action description and the guide now state that a process that cannot write the graph answers a promoting action 403 action_not_offered and offers it in no _actions; tested in openapi (f2c103e67)
status: addressed
---
