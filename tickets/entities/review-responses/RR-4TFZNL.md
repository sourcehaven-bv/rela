---
id: RR-4TFZNL
type: review-response
title: PermitsWorld returning a bare bool collapses store outage into silent empty-200
finding: |-
    VERIFIED. dn37j2-plan.md §3.1 proposes `func (r *Request) PermitsWorld(ctx, world string) bool`, modelled on holdsPermission (resolver.go:265-267).

    Wrong model. The world grant is a READ capability, and this package's read paths are error-carrying deliberately: PermitsRead returns (bool, error) (request.go:145), PermitsReadMany returns (map, error) (request.go:163), and listPushdown fails closed AND propagates (visibility/pushdown.go:74-79).

    PermitsWorld as specified calls r.Globals(ctx) -> walkMembers (resolver.go:79-104), which on a graph error returns a PARTIAL result rather than erroring. So under a store outage PermitsWorld returns false for a principal who genuinely holds the grant. Fail-closed — fine so far. But per binding Q4 the caller must render false as EMPTY 200, indistinguishable from 'no grant'.

    Net effect: a store outage becomes a silently empty page with NO operator signal and NO 500. That is the wrong failure mode for infrastructure failure, and it is the same class of error Q4 already resolved for the unknown-vs-denied distinction — two failure modes deserve two answers.

    FIX: `PermitsWorld(ctx, world string) (bool, error)`. WorldSurfaceFor distinguishes DENIAL (empty 200, per Q4) from INFRASTRUCTURE ERROR (500). Same discipline Q4 applied.
severity: significant
status: open
---
