---
id: RR-CDQ8O
type: review-response
title: No inter-process locking on user-state writes; concurrent rela processes corrupt last_seen_version
finding: 'state.FSKV.Put is raw WriteFile with no lock. Plan adds atomic .tmp+rename (crash-safe) but not concurrent-writer-safe. Realistic concurrency: data-entry server + scheduler + CLI invocation + rela mcp = 4 processes. last_seen_version is rollback-defense: two procs observing N, both calling StoreVersion(N+1), losing one write is a security-degradation bug.'
severity: critical
resolution: 'Use github.com/rogpeppe/go-internal/lockedfile (what Go itself uses) for file-level locks. Lock compound ops: StoreVersion read-compare-write, reseal-sentinel read-check-write. For independent-key writes (ui-state.json, palette.yaml), atomic rename is sufficient since last-writer-wins is acceptable semantics there. Document which keys are locked vs last-writer-wins in service godoc.'
status: addressed
---
