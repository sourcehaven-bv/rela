---
id: RR-4JSUGMP
type: review-response
title: Read-only peer can broadcast stateless frames
finding: '[security] msgBroadcastStateless was relayed to every peer even from a read-only connection; upstream only dropped its document and awareness writes.'
severity: minor
resolution: peer.go drops BroadcastStateless from read-only peers; TestReadOnly_BroadcastStatelessDropped pins it.
status: addressed
---
