/**
 * Maps rela's operator-authored sidebar config onto the component library's
 * `NavGroup[]` / `NavItem[]`.
 *
 * The two models disagree on identity, which is why this is a module with
 * tests rather than an inline `.map()`. A `SidebarItem` has no id: it carries
 * a `label` plus EITHER an `href` or an `action`, and rela's old template
 * keyed rows on `label + (href || action)`. `NavItem.id` is required and is
 * what `activeId` matches against, so it has to be synthesised.
 *
 * A wrong id fails SILENTLY, the same way a wrong table column key does: a
 * duplicate highlights two rows at once, and an unstable one loses the active
 * highlight whenever the list re-renders. Neither raises anything from
 * vue-tsc, eslint or a unit test that only checks rows exist. Hence `navId` is
 * pinned by its own tests and collisions are asserted impossible.
 */
import type { Component } from 'vue'
import type { NavGroup, NavItem, NavItemStatus } from 'rela-components/types'
import type { SidebarGroup, SidebarItem } from '@/types'
import type { NavItemEntry, NavItemList } from '@/api/navItems'
import { entityDetailHref } from '@/utils/entityRoute'
import type { NavEntitiesLookup } from '@/composables/useNavEntities'
import type { PileSummary } from '@/api/piles'
import { pileIcon } from '@/components/piles/pileIcon'

/**
 * The stable identifier for a nav row.
 *
 * Prefixed by kind so a link and an action that share a label cannot collide,
 * and built from the target rather than the label so that retitling an entry
 * in config keeps its identity (and therefore its active highlight).
 *
 * An entry with neither href nor action is malformed config rather than a
 * shape to support. It gets a stable placeholder so the sidebar still renders,
 * and `toNavGroups` makes repeats unique.
 */
export function navId(item: SidebarItem): string {
  if (item.action) return `action:${item.action}`
  if (item.href) return `href:${item.href}`
  return 'item:unnamed'
}

/**
 * Whether an entry has somewhere to go.
 *
 * An entry with neither an action nor a NON-EMPTY href is malformed: the
 * server sends `href: ""` for a nav entry whose kind it does not recognise
 * (rela's own `tickets/data-entry.yaml` has a `graph: true` entry, and
 * `NavigationEntry` has no such field), so an empty string means "no
 * destination", not "the root path".
 *
 * Such an entry MUST NOT be rendered as a link. `RouterLink` resolves `to: ''`
 * against the current location, so the row would match whatever page the user
 * is on and steal the active highlight from the row that should have it — on
 * every route, including ones with no nav entry at all. The old template hid
 * this by rendering nothing for a falsy href; this states the rule instead of
 * relying on a `v-else-if` to imply it.
 */
export function isNavigable(item: SidebarItem): boolean {
  return Boolean(item.action) || Boolean(item.href)
}

/**
 * Converts one rela sidebar group to the library's.
 *
 * `as` carries the COMPONENT for a link, never a name to resolve at render
 * time — the same boundary the icon registry holds. A config string must not
 * be able to choose what gets mounted, so the router link is supplied by the
 * caller (which imports it) rather than named in config.
 */
export function toNavGroups(
  groups: SidebarGroup[],
  routerLink: Component,
  onFlyoutClick?: FlyoutClickHandler,
  statusFor?: NavStatusLookup
): NavGroup[] {
  const seen = new Set<string>()

  return groups.map((group, index) => ({
    // A group heading is optional in rela's config, so the index is the only
    // thing always present to key an unlabelled group on.
    id: group.group ? `group:${group.group}` : `group:${index}`,
    label: group.group,
    ...(group.items_create ? { addLabel: `New ${group.items_create.label}` } : {}),
    // A malformed entry is dropped rather than rendered inert: a row that
    // looks like a link and goes nowhere is worse than an absent one.
    items: group.items
      .filter(isNavigable)
      .map((item) => toNavItem(item, routerLink, seen, onFlyoutClick, statusFor)),
  }))
}

/**
 * Receives the click on an entry that opens a flyout (`open: flyout`). Bound
 * in the capture phase, so it runs before the link's own navigation and can
 * cancel it; see shouldDeferToBrowser for which clicks it must leave alone.
 */
export type FlyoutClickHandler = (item: SidebarItem, id: string, event: MouseEvent) => void

/** Resolves an entry's `status_key` to its current marker, if it has one. */
export type NavStatusLookup = (key: string | undefined) => NavItemStatus | undefined

function toNavItem(
  item: SidebarItem,
  routerLink: Component,
  seen: Set<string>,
  onFlyoutClick?: FlyoutClickHandler,
  statusFor?: NavStatusLookup
): NavItem {
  let id = navId(item)
  // Malformed config can repeat the placeholder, and an operator can point two
  // entries at one target. Duplicate ids would make `activeId` highlight both,
  // so disambiguate rather than let the second row shadow the first.
  if (seen.has(id)) {
    let n = 2
    while (seen.has(`${id}:${n}`)) n += 1
    id = `${id}:${n}`
  }
  seen.add(id)
  const status = statusFor?.(item.status_key)

  return {
    id,
    label: item.label,
    // `icon` is passed through as the NAME. The library resolves it against
    // its own allowlist; rela's registry does the same. Either way the string
    // selects a glyph, never a component.
    icon: item.icon,
    ...(item.action ? { as: 'button' as const } : { as: routerLink, attrs: { to: item.href } }),
    // Still a link to the list's page, so a modified click opens it in a new
    // tab; only the plain click is taken over.
    ...(item.flyout && onFlyoutClick
      ? {
          opensFlyout: true,
          attrs: {
            to: item.href,
            onClickCapture: (event: MouseEvent) => onFlyoutClick(item, id, event),
          },
        }
      : {}),
    ...(status ? { status } : {}),
    ...(item.initial ? { initial: item.initial } : {}),
  }
}

/** Resolves a generated group's `items_key` to its entries, if they have loaded. */
export type NavItemsLookup = (key: string) => NavItemList | undefined

/** The unprefixed href a generated entry opens. */
export type GeneratedItemHref = (group: SidebarGroup, entry: NavItemEntry) => string

/**
 * Where a generated entry goes: the group's entity page for the entry's row,
 * which opens on its first tab, or else the entity itself.
 */
export function generatedItemHref(group: SidebarGroup, entry: NavItemEntry): string {
  if (group.items_page) return `/p/${group.items_page}/${encodeURIComponent(entry.id)}`
  return entityDetailHref(entry)
}

/**
 * Fills each generated group (`items_key`) with its entries, so the rest of
 * the mapping treats them as ordinary items.
 *
 * The ids follow from the hrefs like any other row's, so an entry keeps its
 * highlight while its title changes, and the highlight covers every tab of
 * its page because the tab paths extend the entry's href.
 *
 * A group with no entries is left out, whether its list is empty for this
 * principal or its entries have not loaded or were dropped: a heading over
 * nothing reads as a fault. A group that offers a create stays, since its
 * "+" is how the first entry gets made. When the list holds more rows than the group
 * shows, a last entry links to the whole list.
 */
export function expandGeneratedItems(
  groups: SidebarGroup[],
  lookup: NavItemsLookup,
  hrefFor: GeneratedItemHref = generatedItemHref
): SidebarGroup[] {
  const out: SidebarGroup[] = []
  for (const group of groups) {
    if (!group.items_key) {
      out.push(group)
      continue
    }
    const list = lookup(group.items_key)
    if (!list?.entries.length) {
      if (group.items_create) out.push({ ...group, items: [] })
      continue
    }
    const items: SidebarItem[] = list.entries.map((entry) => ({
      label: entry.label,
      href: hrefFor(group, entry),
      // No glyph: the library renders the badge only where there is no icon,
      // and an entry without one keeps its label aligned with its siblings.
      ...(entry.initial ? { initial: entry.initial } : {}),
    }))
    if (list.truncated && group.items_list) {
      items.push({ label: 'Show all', href: `/list/${group.items_list}`, icon: 'list' })
    }
    out.push({ ...group, items })
  }
  return out
}

/**
 * Replaces each `entities:` entry with one link per matching entity, so the
 * rest of the mapping treats them as ordinary items.
 *
 * A group left with no items is dropped when it held an `entities:` entry,
 * whether nothing matched or the rows have not loaded: a heading over nothing
 * reads as a fault.
 */
export function expandEntityEntries(groups: SidebarGroup[], lookup: NavEntitiesLookup): SidebarGroup[] {
  const out: SidebarGroup[] = []
  for (const group of groups) {
    if (!group.items.some((item) => item.entities)) {
      out.push(group)
      continue
    }
    const items = group.items.flatMap((item): SidebarItem[] => {
      if (!item.entities) return [item]
      return (lookup(item.entities) ?? []).map((row) => ({
        label: row.title,
        href: row.href,
        icon: item.icon,
      }))
    })
    if (items.length) out.push({ ...group, items })
  }
  return out
}

/**
 * The id of the row matching the current route, for `activeId`.
 *
 * Mirrors rela's old `isActive`: a prefix match, so `/list/features/TKT-1`
 * still highlights the `/list/features` row. Longest match wins, since a
 * prefix match alone would light up `/` for every route.
 *
 * Nil: undefined when no row matches, which leaves nothing highlighted.
 */
export function activeNavId(groups: NavGroup[], path: string): string | undefined {
  let best: { id: string; length: number } | undefined

  for (const group of groups) {
    for (const item of group.items) {
      const to = item.attrs?.to
      if (typeof to !== 'string') continue
      if (path !== to && !path.startsWith(`${to}/`)) continue
      if (!best || to.length > best.length) best = { id: item.id, length: to.length }
    }
  }

  return best?.id
}

/** The id of the sidebar's Piles group. */
export const PILES_GROUP_ID = 'group:piles'

/** The id prefix of a pile's sidebar row; the rest is the pile id. */
export const PILE_NAV_PREFIX = 'pile:'

/** The sidebar row id of a pile, shared by the sidebar and the flyout. */
export function pileNavId(id: string): string {
  return `${PILE_NAV_PREFIX}${id}`
}

/**
 * The Piles group (TKT-K3RJLH): one row per pile, with its icon and its count
 * of readable items, and an add control for a new pile.
 *
 * A row opens the pile in the flyout rather than navigating, so it is a
 * button that claims `opensFlyout`. The group is shown even with no piles,
 * because its add control is how the first one gets made.
 */
export function pilesNavGroup(piles: readonly PileSummary[]): NavGroup {
  return {
    id: PILES_GROUP_ID,
    label: 'Piles',
    addLabel: 'New pile',
    items: piles.map((p) => ({
      id: pileNavId(p.id),
      label: p.name,
      icon: pileIcon(p.icon),
      as: 'button' as const,
      opensFlyout: true,
      ...(p.count > 0
        ? { status: { tone: 'info' as const, label: `${p.count} item${p.count === 1 ? '' : 's'}`, count: p.count } }
        : {}),
    })),
  }
}
