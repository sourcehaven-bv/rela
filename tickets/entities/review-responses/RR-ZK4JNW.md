---
id: RR-ZK4JNW
type: review-response
title: Parity test skips the chain-position check when the reference has none
finding: When resolutionRuleAt returned a nil position the resolver ChainPosition was never compared.
severity: minor
resolution: The parity test now asserts ChainPosition is 0 when the reference reports no position.
status: addressed
---
