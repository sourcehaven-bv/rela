---
id: RR-J0MY0JC
type: review-response
title: Peer route allowlist admits nested underscore sub-resources
finding: 'peerRouteAllowed refused only a first segment starting with _, so GET /api/v1/<plural>/<id>/_export, _presence, _channels, _actions and _attachments passed for a peer key: external commands, presence recording and chat reads. Found by both reviewers.'
severity: significant
resolution: peerRouteAllowed now refuses any segment that starts with _ at any depth (and empty segments). TestPeerRouteAllowed pins _export, _presence, _channels, _actions, _attachments, _history and _lease as refused. Docs and the 403 detail say so.
status: addressed
---
