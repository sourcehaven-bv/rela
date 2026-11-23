---
paths:
  - "internal/configedit/**"
  - "cmd/rela-server/**"
---

# Configure space

- **Configure space** (TKT-F5NGMG, `internal/configedit`, mounted by
  `cmd/rela-server`). Two rules carry its safety.

  _The allowlist is per key and default-locked._ `editable` lists what the
  browser may create or change; everything else is refused with 422, and
  `TestAllowlist_ClassifiesEveryKey` fails on a config key that is in neither
  `editable` nor `locked`. Removing a whole item is allowed because removal
  takes code away, EXCEPT when the item holds a `protective` key (a guard, a
  `permission:`, an upload `accept:`/`scan`): removing the item and re-adding
  it without the key would loosen a restriction through two allowed steps.
  For the same reason a guarded transition's `from`/`to` are `pinned`, as
  are the trigger and scope keys of an item holding Lua or an ACL bypass. A
  new locked key that restricts rather than runs code belongs in
  `protective` too. The allowlist judges a tree, so `Apply` refuses to write
  bytes that do not parse back to exactly that tree (a `<<` merge key was
  the bypass that rule closes).

  `flows.yaml` is open for `triggers:` only (TKT-ZT2KREN). Triggers fire in
  document order, so `ordered` refuses moving a kept trigger and puts a new
  one last, and a pin's `order` keeps a scripted trigger's outputs exactly as
  read. A trigger output reads and writes as a cascade without per-record
  ACL checks, so every save that changes `flows.yaml` or `schema.yaml` asks
  the `TriggerChecker` (`appbuild.CheckTriggerACL`, wired in
  `cmd/rela-server/configure.go`) to judge the triggers against the parsed
  `acl.yaml` and the editor's own rights. It refuses scripts, template
  variants, `if_exists: replace`, fixed `relate.to` targets, reads of fields the policy hides, created records
  read more widely than their source, writes the policy decides access by
  (before and after), writes the editor could not make or a flow locks,
  and the same through the triggers a created record fires. When it cannot judge an
  edit, the save is refused. configedit may not import `acl`, so the
  interface lives in configedit and the server supplies it. The `flows:`
  section must parse the same after a save, or flow versions would change.

  A dashboard card that a `capture:` task in `schedules.yaml` stores runs
  as that task's `run_as` and is read by every reader of the owner, so
  `captureProblems` refuses a save that changes its definition: every card
  with the title, its list, the folded property, every query scope and the
  faces of the list's type and its parents, the worlds and default world, and every
  property or relation definition named in the card, list, folded property
  or scopes, followed transitively (TKT-6OYQO79). An unparseable `schedules.yaml` refuses the save.

  Only an interactive principal configures: `mayConfigure` refuses a
  `principal_type` other than empty or `user`, because a client type no
  baseline matches is unrestricted by the ceiling.

  _A save replaces the server, it does not mutate it._ `cmd/rela-server`
  builds a complete new generation (`appbuild.Discover` + the app) from the
  written files, migrates it, swaps the root handler, and only then retires
  the old one in the background (scheduler hand-over, bounded drain,
  `svc.Close`). Do not make `App` fields swappable instead. dataentry takes
  the Configure API as a plain `http.Handler` and must not import
  `configedit` (arch-lint).
