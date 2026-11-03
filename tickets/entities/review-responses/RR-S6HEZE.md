---
id: RR-S6HEZE
type: review-response
title: Every rule x row evaluation rebuilds the bindings, including all predicate functions
finding: 'internal/realtime/trigger.go:408-434. eval calls predicate.NewBindings and predicatefns.Bind(b, now) for every row and rule. A reconcile over 100k entities with 5 rules does this 500k times (and process() repeats it for candidates). Fix: bind functions once per round (now is fixed per round) and only SetVar("entity", ...) per row, if predicate.Bindings allows it, or cache a per-round base binding and clone it.'
severity: minor
resolution: Predicate functions are bound once per round and only the entity record is set per row.
status: addressed
---
