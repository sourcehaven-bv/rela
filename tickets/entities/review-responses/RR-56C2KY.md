---
id: RR-56C2KY
type: review-response
title: Base URL path prefix silently discarded
finding: newRequest used base.ResolveReference with an absolute path, which replaces the base path entirely. A --remote with a path prefix (https://host/rela/, a proxy mounting the API under a sub-path) would drop the prefix and hit the wrong path, surfacing as a confusing 'record not found' rather than a config error.
severity: critical
resolution: newRequest now JoinPaths raw segments onto the base URL, preserving any prefix. Added regression test TestClient_BasePathPrefixPreserved asserting a /rela/ base yields /rela/api/sync/manifest.
status: addressed
---
