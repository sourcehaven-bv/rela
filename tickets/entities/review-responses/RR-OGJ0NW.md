---
id: RR-OGJ0NW
type: review-response
title: Peer gate errors became target_not_found warnings
finding: collectEdgeWarnings logged a read-gate error and treated the peer as absent. A live peer was misreported as target_not_found and the write went ahead.
severity: significant
resolution: collectEdgeWarnings returns the error wrapped in gateFaultError. writeRelationsValidationError answers it with writeGateError (opaque 500 or 504). TestRelationPeerGateFault_FailsTheWrite pins it and checks the raw error stays off the wire.
status: addressed
---
