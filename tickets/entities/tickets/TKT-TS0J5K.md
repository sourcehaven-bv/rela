---
id: TKT-TS0J5K
type: ticket
title: 'ACL: dedicated authorization-misconfiguration validator / audit insights (escalation foot-guns, dead assignments, un-gated membership)'
kind: enhancement
priority: low
status: backlog
---

Follow-up from TKT-Z8A62F.

While making the membership relation configurable we added two ad-hoc
`slog.Warn` hardening checks in `Policy.Validate` (un-gated membership relation,
empty assignments). These are tolerant-by-design advisories, not a real
analysis.

Idea: a dedicated authorization-misconfiguration validator / audit surface that
gives operators *insight* into escalation foot-guns and dead config, rather than
scattering one-off warnings through `Validate`. Candidate findings:

- Membership relation (default `member-of` or configured) confers an assigned
role but is not gated by `requires_permission` → self-promotion path. Should
cover the **default** member-of too, not just the configured case.
- A role-conferring relation grants a privileged role with no delegate gate.
- Assignments referencing undeclared roles / groups (already warned at load, but
collect into one report).
- Roles that grant write on a type they can't read where no role-relation backs
the read-back (likely-broken submitter pattern).
- Privileged role assigned to `everyone`.
- `membership_relation` set but no edges of that type exist in the graph
(graph-aware check — needs the store, unlike Validate).

Open questions:
- Surface: a `rela acl lint` / `rela analyze` subcommand vs. an `analyze_*`
MCP tool vs. a startup report. Graph-aware checks need store access, so this is
more than a pure-policy `Validate`.
- Severity model (info / warn / error) and whether any should be opt-in
enforcement (load failure) for hardened deployments.

Out of scope of TKT-Z8A62F; tracked separately so the config change stays small.
