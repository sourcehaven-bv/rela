---
id: RR-XKQZ1N
type: review-response
title: Validate default-fill mutates a Policy() contract that declarative.go documents as immutable
finding: declarative.go:75-81 documents the *Policy returned by Declarative.Policy() as 'must be treated as immutable' (RR-9GN3/RR-WTLD). The plan has Validate() mutate the receiver to fill the default. That's fine for the LoadPolicy path (Validate runs before the pointer is shared), but reinforces that correctness must not depend on the mutation having happened (see RR-LFMR7S). Prefer a non-mutating effective-name accessor for the resolver read; if Validate still writes the default for operator-facing echo/printing, that's acceptable since it happens at load before sharing. Keep the two concerns separate.
severity: minor
resolution: 'Plan revised: Validate does NOT mutate the receiver. The effective name is resolved via the read-only membershipRelation() accessor, respecting the Policy()-is-immutable contract (declarative.go:75).'
status: addressed
---
