---
id: RR-H3K4LW
type: review-response
title: typingViews key can collide; it does not match the server's entry identity
finding: 'frontend/src/components/entity/channels/channelView.ts:311 keys a typing view by user + NUL + tool, but the server holds one entry per bucketIdentity, which is user + NUL + org (internal/realtime/ratelimit.go:145). The same user in two orgs, or two org-scoped principals with the same user name and no tool, yields duplicate v-for keys. Fix: key by user, org-independent fields plus since, or by index; the list is tiny and re-rendered per head anyway.'
severity: nit
resolution: typingViews keys by user and since with a counter on a clash, so keys never repeat; test covers one user in two orgs.
status: addressed
---
