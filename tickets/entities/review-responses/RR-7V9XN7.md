---
id: RR-7V9XN7
type: review-response
title: FindPath withheld-path timing side channel — document as accepted residual
finding: Withholding a computed path (hidden intermediate → nil) takes marginally longer than a genuine no-path BFS failure on some graphs; a caller timing find_path could in principle distinguish the two. In-memory graph traversal noise makes this impractical to exploit; not worth constant-time engineering. Document as an accepted residual in the package godoc, mirroring how other indistinguishability invariants note their bounds.
severity: nit
resolution: 'Accepted residual, documented: package godoc will note the FindPath withhold-vs-no-path timing difference and why it is not worth constant-time engineering (PLAN-RR12W4 Approach).'
status: addressed
---
