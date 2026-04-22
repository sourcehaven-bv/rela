---
id: TKT-96VGO
type: ticket
title: Make rela.cache safer when one script serves multiple documents
kind: enhancement
priority: low
status: backlog
---

## Problem

`rela.cache` is namespaced per script path, so two documents configured with the
same `script:` value share a cache namespace. If a doc script writes keys like
`"summary:" .. entry_id` (without including `rela.document.id`), data from one
document overwrites another's on every call.

The guide (GUIDE-data-entry and GUIDE-lua-scripting) warns about this, but
warn-in-docs-only is a footgun. The script-path namespace default is the right
ergonomic for shared helper libraries that want cross-caller cache reuse —
changing it per-document would regress that use case (see RR-I5WME on
TKT-CGBVW).

## Options

**(a) Auto-inject `rela.document.id` into the cache namespace when running in
document mode.** Pro: predictable default, no way to collide across docs. Con:
loses the "shared helper" benefit for document-mode scripts. Con: behavioral
difference between modes is surprising.

**(b) Expose a helper like `rela.cache.scoped(prefix)`** that returns a
cache-like table whose get/set/memoize prepend the prefix. Authors opt in by
writing `local c = rela.cache.scoped("doc:" .. rela.document.id);
c.memoize("summary:" .. id, fn)`. Pro: explicit, composable. Con: one more thing
to teach.

**(c) Leave the guide warning in place.** Pro: zero code change. Con:
footgun-by-design.

## Recommendation

(b). It's ergonomic for the doc case (`scoped("doc:" .. rela.document.id)`)
while keeping the default namespace untouched for shared libraries. `scoped` can
also serve non-document use cases (e.g., per-tenant caching in a custom action).

## Scope

- Add `rela.cache.scoped(prefix string)` to `internal/lua/cache.go`.
- Return a new Lua table whose `get`/`set`/`memoize` close over the namespaced user prefix.
- Update GUIDE-lua-scripting with the new helper and the recommended doc-mode idiom.
- Leave `rela.cache.{get,set,memoize}` at the top level unchanged.

## Out of scope

Changing how the script-path namespace is computed or stored.
