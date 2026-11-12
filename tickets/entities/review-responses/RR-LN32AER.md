---
id: RR-LN32AER
type: review-response
title: TryFlowWorkerLock nil dereference and flowOf on every forked put
finding: The test export TryFlowWorkerLock dereferenced a nil worker. joinDue reads the flow definition on every forked put.
severity: nit
resolution: TryFlowWorkerLock returns an error without a worker. joinDue reads the flow only when some but not all branches arrived and none is pending. For a run on the current flows.yaml version that is a map lookup; only a run on an older version compiles its stored definition, as every other step of such a run already does. Not worth a parameter through every put caller.
status: addressed
---
