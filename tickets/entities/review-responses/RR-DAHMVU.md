---
id: RR-DAHMVU
type: review-response
title: The plan cites sqlite cross-process writes that cannot happen
finding: sqlitestore takes an exclusive sidecar lock (DEC-LFSYNY), so there are no "sqlite writes from another process" (cited in §0, the reconcile section and risk 4). The sqlite `filelock` round lock adds nothing. Use the process-private lock on sqlite and fix the rationale.
severity: minor
resolution: sqlite uses the process-private lock and the rationale is corrected.
status: addressed
---
