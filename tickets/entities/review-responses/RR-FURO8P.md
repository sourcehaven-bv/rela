---
id: RR-FURO8P
type: review-response
title: Startup-only gate misses live metamodel reloads
finding: 'The plan wires the compatibility gate at startup, but the metamodel hot-reloads at runtime: FSLoader subscribes via fsnotify with a debounce and re-resolves includes on every schema.yaml change (internal/metamodel/loader_service.go). An operator editing schema.yaml under a running server introduces shape drift that the gate never classifies — no adoption, no notices, and the GC sweep''s ''skip while needs-migration pending'' check would consult a stale verdict. The gate must also run on the metamodel reload path, and sweeps must consult the latest gate state each tick.'
severity: significant
resolution: 'Amendment A4: the gate subscribes to metamodel hot-reload (FSLoader fsnotify path) and re-evaluates on every reload, publishing its verdict via atomic.Pointer per the state-publish rule; the GC sweep reads the latest verdict each tick. New AC12 covers a mid-run incompatible edit surfacing without restart.'
status: addressed
---
