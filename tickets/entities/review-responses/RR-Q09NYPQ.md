---
id: RR-Q09NYPQ
type: review-response
title: Operator config surfaces can name a remote_credential secret
finding: 'subscription auth.secret, connections.yaml client_secret_secret and the AI key lookup accept a remote_credential. name and would send it to their URL. Same as peer_credential in #2156.'
severity: minor
reason: Operator-authored config, not script reach; a subscription to another rela may need it. Godoc and guides now say no Lua script can read it, not that nothing else can.
status: deferred
---
