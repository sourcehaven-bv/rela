---
id: RR-XYO03L
type: review-response
title: ReadOnlyACL arm hid entries a read-only principal can fully use
finding: 'permitsNavEntry copied authorizeCommand''s ReadOnlyACL deny arm without its justification. authorizeCommand gates shell execution — a write-shaped act, correctly denied under --read-only. permitsNavEntry gates menu links to READ surfaces (list, kanban, dashboard, search), and acl.ReadOnlyACL only implements AuthorizeWrite; it restricts no reads at all. So my stated rationale ("the principal cannot act on anything") was simply false. Worse, ReadOnlyACL carries no identity, so the arm hid gated entries from EVERYONE — `permission:` silently changed meaning from "hide from non-holders" to "hide from all" based on a process-wide flag about writes. The concrete cost: the audit-log entry my own docs use as the example vanishes in post-incident forensic mode, a documented ReadOnlyACL use case.'
severity: critical
resolution: 'Grouped ReadOnlyACL with NopACL and made both SHOW. The reviewer''s framing is right: neither carries a policy, so neither has a permission model to consult — read-only is structurally the same case as no-ACL, not its opposite. The arm stays explicit rather than falling through, because the RR-CWWJGW hazard is real (readGateFromContext returns nopReadGate under read-only, whose HoldsPermission returns true, so falling through would reach the same answer by accident). The godoc now says that plainly instead of asserting a false rationale, and a new test TestNavPermission_ReadOnlyArmIsExplicit pins it by attaching a deny-everything gate: if the arm is removed, the default arm hides the entries and the test fails. Mutation-verified both directions.'
status: addressed
---
