# entitymanager — rules for new code

`entitymanager.Manager` is the "human intent" write path: it runs
automations and validation on top of `store.Store`, emits the audit log,
and consults the ACL. All entity/relation writes go through here — do not
write directly to `store.Store` from a write path, or the audit record
won't be emitted.

## Audit log

Every successful entity/relation create/update/delete/rename is audited as
a JSONL record under `.rela/audit/YYYY-MM-DD.jsonl`. See `docs/audit-log.md`
for the user-facing reference. Rules for new code:

- **New write paths inherit audit automatically.** Any code calling
  `Manager.{Create,Update,Delete,Rename}{Entity,Relation}` produces a
  record without further wiring. Do not bypass Manager.
- **New entry-point binaries stamp Principal at startup**, once:
  `ctx = principal.With(ctx, principal.Principal{User: principal.SystemUser(), Tool: principal.ToolXxx})`.
  Use a `principal.ToolCLI`/`ToolMCP`/`ToolDataEntry`/`ToolScheduler`/
  `ToolDesktop` constant — string literals won't surface typos until the
  entry-point smoke test catches them.
- **Engine-initiated paths stamp `triggered_by`.** Scheduler tasks wrap the
  per-task ctx with `audit.WithTriggeredBy(ctx, "schedule:"+task.Name)`; the
  autocascade runner does the analogous thing for cascades. Direct user
  actions leave `triggered_by` empty.
- **Lua bindings do not expose audit *rewrite* primitives.** A Lua script
  must not be able to change the Principal or triggered_by a write is
  attributed as — attribution always derives from the caller's context
  inside the write bindings, never from anything the script controls.
  Do not register `rela.audit`, `rela.audit.with_principal`, or
  `rela.audit.with_triggered_by` on the runtime — those would be rewrite
  vectors. Guarded by `internal/lua/audit_spoofing_test.go`.

  `rela.principal` **is** exposed (TKT-5U6NRR) — but **read-only**: a frozen
  `{user, tool}` table (`__newindex` raises, `__metatable` locked) read from
  the request context. It only *reads* the acting identity (so write-path
  automations can attribute relations like `created-by` to the real
  submitter); it is not a rewrite hook, and the write bindings ignore it
  entirely. Reading the identity cannot forge attribution, so it does not
  weaken the spoofing defense — the test pins both the read-only contract and
  the can't-forge invariant. Do not add a *setter* or any path from a
  script-controlled value into audit attribution.
- **Constructor takes `Audit` as a required collaborator.**
  `entitymanager.Deps.Audit` and `appbuild.New` reject nil. Tests use
  `audit.Nop{}` (explicit opt-out) or `audit.NewMemory()` when asserting.

## Authorization (ACL)

The ACL is a required collaborator via `Deps.ACL`; structured 403s surface
in `internal/dataentry`. Three production implementations live in
`internal/acl`:

- `NopACL` — allow-all; default when no `acl.yaml` is present.
- `ReadOnlyACL` — deny-all; wired via `rela-server --read-only`.
- `Declarative` — policy-driven, composed with a `Policy` from `acl.yaml`.

Consumer-side interface rule: code calling into the ACL declares the
narrowest contract it needs at the call site, not `acl.ACL` in full.
`entitymanager` is the exception — it owns the constructor field so the
wiring boundary is explicit.

- **Don't run user-supplied Lua on the read path.** ACL gates and filters
  evaluate against declarative policy (`acl.yaml`) and the graph; Lua
  participates only at *write time* via the automation engine. Per-row Lua
  on reads is the perf cliff every comparable system regrets — see
  `.ignored/acl-design.md`.

See `docs/server-security.md` (schema reference), `.ignored/acl-design.md` (design
rationale, four-layer model: users → groups → roles → local roles), and
`docs/audit-log.md` (the `denied-write` audit op).
