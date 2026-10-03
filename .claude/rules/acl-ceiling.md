---
paths:
  - "internal/acl/**"
  - "internal/aclmap/**"
---

# ACL client attenuation

- **Restrictions compile at LOAD time; the evaluator has no denial primitive.**
  Client attenuation (`client_baselines` / `scope_grants`, TKT-IAC8TX) restricts
  a client below the user it acts as, but it does so by compiling into plain
  allowlists when `acl.yaml` loads — `redact: {person: [salary]}` becomes
  "person's permitted fields, minus salary". `decideFromAttrs`, `readQuery`,
  `grantsPermission` and `FieldVerdicts` keep seeing allowlists, so DEC-RG878's
  additive union semantics are intact. **Do not add a runtime deny.**
  `ReadQuery` compiles to a `store.GraphQuery` pushed into SQL, so a runtime
  denial would have to become a SQL predicate in every backend, and every
  evaluation path plus all of `internal/aclmap` would need re-deriving.

  The clamp point is `Request.roleFor` — every evaluation path resolves role
  names through it, so reaching into `policy.Roles[...]` directly from a new
  path silently bypasses the ceiling. A guard test (`ceilingguard_test.go`)
  scans the package and fails on that; it uses an exemption list, so a new file
  must be clean or explicitly exempted.

  A ceiling only ever NARROWS (`effective = user_grants ∩ (baseline ∪ scopes)`),
  so a bug fails toward less access — except in the compilation step, which is
  why that has direct unit tests rather than only end-to-end ones.

  **What this rule does NOT forbid**: adding a new allowlist DIMENSION to the
  compiled result. The prohibition is on subtractive evaluation — a `deny` the
  evaluator applies per row — not on the query carrying more allowlists.
  `ReadQueryResult` already carries a type verdict and a composed `GraphQuery`;
  a per-face allowlist (TKT-FACEREAD) is the same shape: computed from grants at
  compile time, pushed down as an additional predicate, still additive. It costs
  one predicate per backend, not a re-derivation of the evaluator.
