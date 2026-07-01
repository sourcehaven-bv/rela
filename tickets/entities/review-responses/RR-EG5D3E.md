---
id: RR-EG5D3E
type: review-response
title: A1 fires High on a read-only assigned role (false positive; asymmetric with A2)
finding: 'A1 (tier_a.go:50) gates on assignsAnyDeclaredRole (ANY declared role), but A2 (tier_a.go:81) gates on isPrivileged. Per RR-LXI3NW/RR-UR0LJU, granting yourself a read-only role is a visibility choice, not escalation. So default member-of + assignments:{eng: reader} where reader is read-only fires A1 High (''self-promotion hole'') incorrectly — the exact trust-eroding false positive the design fought. The docs (acl-security.md:73-74) describe A1 as conferring ''a privileged role'', disagreeing with the code. The CLI/unit tests use ''editor'' which is privileged via Create, so the read-only case is never exercised. Fix: gate A1 on a new assignsAnyPrivilegedRole predicate (reuse isPrivileged), symmetric with A2; add a read-only-group negative test asserting zero A1.'
severity: significant
resolution: A1 now gates on assignsAnyPrivilegedRole (reuses isPrivileged), symmetric with A2 — a read-only assigned role no longer trips A1. Detail/docs updated to 'a privileged group role'. Added TestAudit_A1_ReadOnlyGroup_NoFinding asserting zero A1 for a read-only group.
status: addressed
---
