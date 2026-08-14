---
id: DOCS-O3LYQN
type: docs-checklist
title: 'Documentation: Client attenuation (TKT-IAC8TX)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] New exported types and functions carry godoc
- [x] Non-obvious decisions explain WHY, not just what

The load-time-compilation decision and the matcher-not-expanded-set choice are
documented at the top of `internal/acl/ceiling.go` and `ceilingcompile.go`
respectively, because both are the kind of thing a future reader would
"simplify" into a runtime denial or a metamodel expansion — which is exactly the
fail-open direction.

## Project Documentation

- [x] `CLAUDE.md` — added a standing rule: "Restrictions compile at LOAD time;
the evaluator has no denial primitive." Names the clamp point
(`Request.roleFor`), the guard test, and why a runtime deny would force
re-deriving `ReadQuery`'s SQL pushdown.
- [x] `internal/acl/ceiling.go` — package-level commentary on the invariant,
the disjointness rule, and why there is no combination semantics.
- [x] `docs-project/entities/guides/GUIDE-acl-overview.md` — new "Restricting a
client below its user" section: the invariant, baseline selection, the two
spellings table, scopes, and what it does NOT do.
- [x] `docs-project/entities/guides/GUIDE-acl-security.md` — new "Client
attenuation" section: trust boundary, the inverted direction of harm (dropping a
claim removes the ceiling), why this does not contradict the additive model, and
the audit rules.
- [x] `docs/` regenerated via `just docs`; `just docs-check` green.

## Correction made

The security guide previously said ACL evaluation is additive "and no rule can
subtract". This feature made that partially untrue, so the claim was corrected
rather than left to rot — it now explains why org enforcement still cannot work
the same way (it needs a per-row predicate, which the read path deliberately
lacks, whereas a ceiling compiles against a claim value set the operator
enumerates).

## External Documentation

- [x] ~~API reference~~ (N/A: no new HTTP endpoint or wire field; the ceiling
changes what existing endpoints return, not their shape)
- [x] CLI: `rela acl map --as/--scope` flags carry help text; new audit rules
(A11/A12/A13/B8/B9) emit their own `fix:` guidance.

## Verification

- [x] The documented `acl.yaml` example is pinned by a doc-drift test
(`TestDocs_AclOverviewClientAttenuationExampleWorks`) that asserts the PROSE
CLAIMS, not merely that the YAML parses — a wrong ACL example gets copied into a
deployment where someone believes a client is restricted.
- [x] The documented `rela acl map --as` console output is a verbatim copy of a
real run against a throwaway project, not written from memory. (My first draft
had the route format wrong; running it caught that.)
