---
id: RR-93HF8G
type: review-response
title: One refused plural blocks every other store schema change in the same reload
finding: 'internal/store/fsstore/schemas.go:44-62. SetSchemas is all-or-nothing, so a reload that changes one held type''s plural and also adds a type leaves the store refusing the new type and still holding the removed ones, until restart. TestMCPServices_ReloadWithMovedPluralKeepsServing says the rest of the schema still applies, which is true for the services but not for the store. Every later reload keeps failing as long as the type has entities. The only signal is slog.Warn, which the MCP client never sees. Fix: keep the previous plural for a held type whose plural changed, apply everything else, and return or log a warning naming the type.'
severity: minor
resolution: SetSchemas applies every added and removed type, keeps the old plural for a refused type and returns an error naming the refused types.
status: addressed
---
