---
id: FEAT-G4VO53
type: feature
title: Generated deployment documentation (rela docs)
description: 'Generate per-deployment end-user + operator/support documentation for a rela project. REFRAMED 2026-07-21 (see RES-EK7LSA addendum): phase 2 is a doc LANGUAGE — markdown authored by a human with mechanical fragments pulled from the schema/graph via Lua islands (```rela blocks / `rela` inline echoes), NOT a static push-generator. Phases: 1a/1a.5/1b doc-fields (DONE); phase 2 = Tier A (language + resolvers: typeref/values/relations/graph/lifecycle/entity/count/roles_matrix + memstore seed, no browser); phase 3 = Tier B (screenshot island via Playwright).'
status: proposed
---

# Generated deployment documentation (`rela docs`)

Generate per-deployment end-user + operator/support documentation from a rela
project's `metamodel.yaml` + `acl.yaml`, as one Markdown file (PDF-convertible;
mermaid state diagrams). Replaces hand-maintained docs that drift from the
schema.

Two audiences (per Diátaxis, the schema produces *reference* + inferred
*how-to*; human prose supplies the *explanation* layer via new doc-fields):
- **End users** — what the entity types are, their fields (with per-value
meaning), how they relate, the lifecycle (state machines as diagrams + narrated
transitions), and what happens automatically (automations).
- **Operators / support staff** — the role model: who can do what, who can
perform which guarded transitions, what rules must hold (GitLab-style role ×
entity capability matrix).

Delivered in phases (see research RES-EK7LSA):
- **Phase 1a** — metamodel doc-fields (top-level description, per-enum-value
descriptions, transition help).
- **Phase 1b** — ACL role descriptions.
- **Phase 2** — the `rela docs --output-dir` generator (depends on 1a + 1b).

New top-level `rela docs` command; depends only on `internal/metamodel` +
`internal/acl`; mirrors `internal/cli/schema.go`'s metamodel walk.
