---
id: RR-5IFZ23
type: review-response
title: 'Proposal is unbuildable as written: bypass_acl registration is welded to elevated WRITES, which the ticket lists as a non-goal'
finding: |-
    The ticket proposes elevated READS for document renders and lists elevated writes as an explicit non-goal. The current code cannot express that combination at two independent points.

    (1) runtime.go:709-718 registers rela.bypass_acl ONLY inside `if allowWrites {`, and only when `r.deps.ElevatedManager != nil` — the elevated WRITE handle. A runtime with an ElevatedReader but no ElevatedManager never gets the binding, so the closure the ticket depends on does not exist.

    (2) luascriptrunner.go:149-160 sets deps.ElevatedReader only INSIDE the `ep.Elevated()` branch, with the comment: 'the elevated READ handle is set under the SAME two keys, inside the same branch — never on its own, so read elevation cannot outlive or precede write elevation.' That is a deliberate invariant, not an accident.

    A document render additionally has no Mutator: LuaScriptRunner.Run requires a non-nil autocascade.Mutator and rejects nil up front, and the whole two-key gate hangs off the Mutator implementing ElevatedProvider. Document renders go through Engine.ExecuteDocument / ExecuteStandaloneDocument -> runDocumentScript (list_document.go:58), which never touches LuaScriptRunner at all. So the ticket's 'reuse NewLuaScriptRunnerWithElevatedReads' is not a small rewiring — that constructor is on the cascade path, and documents are not on it.

    Delivering read-only elevation therefore requires EITHER decoupling read elevation from write elevation in lua/runtime.go and script/luascriptrunner.go (breaking a stated invariant, and widening the surface so a future caller can grant reads without writes), OR granting document renders an elevated write handle they must never use (contradicting the ticket's non-goal and handing a render surface a mutation capability). Both are design decisions well beyond 'extend the existing opt-in', and neither is named in the ticket.
resolution: |
  Resolved by RES-XZBZXB: the blocker was overstated and the decoupling is small.

  CORRECTION to this finding: document renders are ALREADY writer runtimes.
  App.luaWriteDeps() (app.go:321) sets EntityManager, and runDocumentScript
  (list_document.go:85) calls NewWriterRuntime -> lua.NewWriter ->
  newRuntime(allowWrites=true). So the `if allowWrites` condition this finding
  cited as an obstacle is already TRUE on the document path. Only the second
  condition, `deps.ElevatedManager != nil` (runtime.go:713), actually blocks.

  Read and write elevation are already structurally separate inside lua:
  registerElevatedReads (runtime.go:1951) takes only `er EntityReader`, and
  readGuard already checks `er == nil` independently of the mutator. The
  luascriptrunner.go:149-160 coupling is on the CASCADE wiring path, which
  document renders never touch.

  Read precisely, the stated invariant ("read elevation cannot outlive or
  precede write elevation") guarantees elevation never appears WITHOUT operator
  opt-in; it does not assert reads are meaningless alone. A read-only elevation
  reachable only through its own explicit document opt-in satisfies that.

  DECIDED APPROACH (Option A): register bypass_acl when EITHER handle is
  present, and have newElevatedHandle register write methods only when em !=
  nil. A document render's `admin` table then exposes exactly get_entity /
  list_entities / get_relations; admin.delete_entity is "attempt to call a nil
  value" — reads-only becomes structural rather than promised.

  Residual risk, carried into planning: this widens a deliberately narrow gate,
  so a future wiring site could grant reads without writes by accident. Mitigate
  by keeping nil-reader = deny, requiring explicit document config for the
  opt-in, and pinning with a test asserting no write method exists on a
  read-only handle.
severity: critical
status: addressed
---
