---
id: RR-N2MAEJU
type: review-response
title: Peer-side key is readable by scripts
finding: The live key on the peer was stored under an ordinary secret name, readable by any script granted it or all secrets. Security review.
severity: minor
resolution: 'New reserved prefix peer_credential.: ScriptReadable refuses it, --token-secret accepts it, the docs and CLI output use it. Tested in secrets and cli.'
status: addressed
---
