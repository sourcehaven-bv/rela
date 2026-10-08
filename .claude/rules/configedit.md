---
paths:
  - "internal/configedit/**"
  - "cmd/rela-server/configure*.go"
  - "cmd/rela-server/serve.go"
---

# Configure space

The Configure space (TKT-F5NGMG, `internal/configedit`, mounted by
`cmd/rela-server`) edits `schema.yaml` and `data-entry.yaml` from the browser.
Two rules carry its safety.

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
