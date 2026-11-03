---
id: RR-3RJCID
type: review-response
title: Raw mention capability reachable from surfaces
finding: An optional store capability found by type assertion invites MCP/Lua to bypass the row gate.
severity: minor
resolution: store.MentionIndex is an optional capability, but internal/archguard/mentionindex_test.go fails the build if any package outside internal/store and internal/visibility calls it. Every surface goes through visibility.Resolver.MentionedBy.
status: addressed
---
