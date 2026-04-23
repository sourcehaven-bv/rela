---
id: RR-WVX2X
type: review-response
title: All() returns shallow copy; Route.Params shares backing array
finding: internal/frontendroutes/routes.go:57-62 All() copies the top-level slice but each Route.Params slice is still the backing array from the private routes variable. Mutating All()[0].Params[0].Lua mutates the catalog. No caller does this today, but the comment promises 'callers can mutate it freely'. Update the comment to 'treat contents as read-only' or deep-copy Params during the duplication.
severity: minor
resolution: 'Doc comment on frontendroutes.All() updated: ''The returned slice itself is a fresh copy, but inner slices (Params) share backing arrays with the catalog — treat route contents as read-only.'' No deep copy because no caller needs to mutate.'
status: addressed
---
