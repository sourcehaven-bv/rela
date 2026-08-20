---
id: RR-FURO8P
type: review-response
title: Startup-only gate misses live metamodel reloads
finding: 'The plan wires the compatibility gate at startup, but the metamodel hot-reloads at runtime: FSLoader subscribes via fsnotify with a debounce and re-resolves includes on every schema.yaml change (internal/metamodel/loader_service.go). An operator editing schema.yaml under a running server introduces shape drift that the gate never classifies — no adoption, no notices, and the GC sweep''s ''skip while needs-migration pending'' check would consult a stale verdict. The gate must also run on the metamodel reload path, and sweeps must consult the latest gate state each tick.'
severity: significant
resolution: 'Amendment A4, refined during implementation: investigation showed there is NO server-side metamodel hot-reload today (dataentry''s rebuildState only ever fires for data-entry.yaml; FSLoader.Subscribe has no runtime caller), so a running server serves a boot-time metamodel and the gate verdict ages exactly as the metamodel does — they can never disagree. The Gate is built for re-evaluation (verdicts publish via atomic.Pointer, GC reads the latest each tick, TestGate_ReEvaluateOnReloadUpdatesVerdict pins it) and is re-evaluated per process start and inside every `rela migrate` command; when live metamodel reload lands, the reload path calls gate.Evaluate with the new metamodel (documented in appbuild/datamigration.go).'
status: addressed
---
