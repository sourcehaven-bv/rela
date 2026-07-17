---
id: RR-7UXWNA
type: review-response
title: who-can read must use the real read path, not a computeForEntity reimplementation
finding: 'The plan proposes extending computeForEntity to filter attributions by RoleDef.Read to source read provenance. But the runtime read path is readQuery/PermitsRead (readquery.go), which grants read via THREE mechanisms including a HasInbound GraphQuery over role-conferring relations with its own independent ancestor-BFS (naive.go expandSet). computeForEntity uses a separate BFS (resolver.go ancestors) and separate edge-orientation logic (HasEdge probes). These are parallel reimplementations maintained independently. A who-can read built on extended computeForEntity can therefore report a principal CANNOT read when readQuery would grant it (false negative). For a confidentiality-attestation tool this is the worst failure mode: a security officer wrongly attests an entity is unreadable. Fix: compute who-can read from readQuery''s OWN building blocks (the AllowAll global-role set + the `conferring` relation-type set at readquery.go:28-48) or drive per-principal decisions through PermitsRead, so the tool''s answer is guaranteed to agree with the runtime. Do NOT reconstruct read on the computeForEntity path.'
severity: critical
resolution: 'Plan revised: who-can read is now specified to go through the runtime read path (PermitsRead / a helper sharing readQuery''s AllowAll+conferring structure), NOT a computeForEntity reconstruction. Added an acceptance criterion + a read-vs-runtime conformance test asserting who-can read E == {p : PermitsRead(E)} including a reader reachable only via the HasInbound/conferring path. Implementation must satisfy that test.'
status: addressed
---
