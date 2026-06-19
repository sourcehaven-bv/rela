---
id: RR-48FJJE
type: review-response
title: ApplyRelation endpoint lookup masked transient errors as ErrEntityNotFound (infinite retry)
finding: apply.go endpoint lookups wrapped EVERY error as ErrEntityNotFound. The sync apply layer's retry loop keys on ErrEntityNotFound = 'endpoint not applied yet, retry next pass'. So a transient pgstore error on an endpoint that ACTUALLY EXISTS reports 'missing endpoint' → the apply layer retries forever waiting for an entity already present. CreateRelation has the same collapse but it's user-driven (human stops); ApplyRelation feeds an automated retry loop, which makes it dangerous.
severity: critical
resolution: Extracted requireEndpoint(ctx,id,role) that distinguishes store.ErrNotFound (→ErrEntityNotFound, retry) from any other error (→wrapped underlying error, fail closed). Regression TestApplyRelation_EndpointProbeFailsClosed seeds real endpoints, makes the probe flake, asserts the error is NOT ErrEntityNotFound and the sentinel propagates. requireEndpoint at 100% coverage.
status: addressed
---
