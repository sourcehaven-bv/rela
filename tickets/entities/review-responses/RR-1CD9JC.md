---
id: RR-1CD9JC
type: review-response
title: '[security] Paths under /tmp are bound from the shared host /tmp'
finding: A listed /tmp/clamd.socket is bound from world-writable /tmp; while absent any local user can create it, and a fake socket answering OK would make scans fail open. Warn/refuse paths under a world-writable sticky dir, or document that the socket must live in a root- or daemon-owned directory.
severity: nit
resolution: 'transforms.md now says not to list a socket in a directory other users can write to such as /tmp, and why (a fake socket answering clean lets infected uploads through). Not refused in code: CI and local tests legitimately bind scratch sockets under the temp dir, and a root-owned /run path is the documented default.'
status: addressed
---
