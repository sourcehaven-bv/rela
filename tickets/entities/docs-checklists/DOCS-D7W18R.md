---
id: DOCS-D7W18R
type: docs-checklist
title: 'Docs: Permission-based dashboard card filtering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc / comments on new exported and non-obvious code

`DashboardCard.Permission` carries a doc block mirroring
`NavigationEntry.Permission`. `permitsGatedUIElement` carries the full
"presentation only" policy godoc (moved off the thin `permitsNavEntry` wrapper
so the function everyone actually calls is the documented one), and names
`authorizeCommand` as what to use if you want an authorization check.
`v1.DashboardResponse` documents why the endpoint is separate from `/_config`
and why `Cards` is always non-nil.

## Project Documentation

- [x] `internal/dataentry/CLAUDE.md` updated

Renamed the section to "Presentation filtering (`permission:` on a nav entry or
dashboard card)" and extended every rule to both surfaces: the UX-not-boundary
rule, the do-not-filter-`/_config` rule (now with the reason the dashboard
needed its own endpoint), the load-bearing ReadOnlyACL arm, the closed switch,
and two new rules — keep the grep guard's allow-lists short, and a third gated
surface *shares* the predicate rather than copying it.

**Corrected a stale reference while there:** the section named the read-only
canary `TestNavPermission_ReadOnlyHides`, which does not exist and is actively
misleading — it says "Hides" where the pinned behavior is "Shows". Now names the
real tests, `TestNavPermission_ReadOnlyShowsEverything` and
`_ReadOnlyArmIsExplicit`.

Also updated the cross-reference in the Documents section, which previously
mentioned only the sidebar as the opt-in filtering exception.

- [x] `docs/acl-security.md` updated

Extended the "Sidebar menu structure is principal-independent" section to cover
dashboard cards under the same rationale ("buys tidiness, not protection"), and
stated explicitly why the card list needs its own endpoint: `/_config` is
principal-independent by design and must stay that way. "Do not collapse them."

## External Documentation

- [x] `docs/data-entry.md` updated

Added `permission` to the Card Options table plus a "Hiding cards a user cannot
use" section with a worked YAML example, the not-a-security-control framing, the
no-`acl.yaml`/`--read-only` behavior, and the all-filtered empty state.

Includes the RR-2KZEXF gotcha as a callout: **permission names are not
validated**, so a typo yields a card nobody can see with no error — check the
roles' `permissions:` list first. Names that this applies equally to commands,
documents and nav entries, so the reader learns the general shape rather than
one instance.

## Not Applicable

- [x] ~~`docs/metamodel.md`~~ (N/A: this is a `data-entry.yaml` key, not a metamodel feature)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no command changes)
- [x] ~~`README.md`~~ (N/A: no project-level change)
