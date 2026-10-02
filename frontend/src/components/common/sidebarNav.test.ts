import { describe, it, expect } from 'vitest'
import { RouterLink } from 'vue-router'
import type { SidebarGroup } from '@/types'
import type { NavItemList } from '@/api/navItems'
import { activeNavId, expandGeneratedItems, navId, toNavGroups } from './sidebarNav'

/**
 * The nav id is the thing here that fails silently when it is wrong.
 *
 * `activeId` matches on it, so a duplicate highlights two rows and an unstable
 * one drops the highlight on re-render — neither of which raises anything.
 * These tests therefore assert the IDS themselves and the properties that make
 * them safe (distinctness, stability), not merely that a conversion happened.
 */
describe('navId', () => {
  it('distinguishes an action from a link of the same name', () => {
    expect(navId({ label: 'Sync', action: 'sync' })).not.toBe(
      navId({ label: 'Sync', href: 'sync' })
    )
  })

  it('cannot collide with the placeholder a malformed entry gets', () => {
    // Asserted because the kind prefix is otherwise unpinned: the action id
    // carries its own prefix, so dropping the href one still leaves the two
    // distinct and the test above passes anyway. A bare href is one
    // `item:unnamed` away from colliding with malformed config, silently.
    expect(navId({ label: 'x', href: 'unnamed' })).not.toBe(navId({ label: 'y' }))
    expect(navId({ label: 'x', href: 'item:unnamed' })).not.toBe(navId({ label: 'y' }))
  })

  it('ignores the label, so renaming an entry keeps its identity', () => {
    // The label is operator-facing prose. If it fed the id, retitling an entry
    // in config would silently drop its active highlight.
    expect(navId({ label: 'Features', href: '/list/features' })).toBe(
      navId({ label: 'Epics', href: '/list/features' })
    )
  })

  it('prefers the action when an entry carries both', () => {
    // rela's template tests `item.action` first, so an entry with both renders
    // as a button. The id must agree with what is rendered.
    expect(navId({ label: 'x', action: 'run', href: '/x' })).toBe('action:run')
  })
})

describe('toNavGroups', () => {
  const groups: SidebarGroup[] = [
    {
      group: 'Work',
      items: [
        { label: 'Features', href: '/list/features' },
        { label: 'Sync', action: 'sync' },
      ],
    },
  ]

  it('gives every item a distinct id', () => {
    const ids = toNavGroups(groups, RouterLink).flatMap((g) => g.items.map((i) => i.id))
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('keeps ids distinct when config repeats a target', () => {
    // An operator can point two entries at one href. Left alone they would
    // share an id and `activeId` would highlight both.
    const dupes: SidebarGroup[] = [
      {
        items: [
          { label: 'A', href: '/x' },
          { label: 'B', href: '/x' },
          { label: 'C', action: 'run' },
          { label: 'D', action: 'run' },
        ],
      },
    ]
    const ids = toNavGroups(dupes, RouterLink).flatMap((g) => g.items.map((i) => i.id))
    expect(new Set(ids).size).toBe(4)
  })

  it('renders a link as the router component and an action as a button', () => {
    const [group] = toNavGroups(groups, RouterLink)
    const [link, action] = group.items

    // The component itself, never a name: a config string must not be able to
    // choose what gets mounted.
    expect(link.as).toBe(RouterLink)
    expect(link.attrs).toEqual({ to: '/list/features' })

    expect(action.as).toBe('button')
    expect(action.attrs).toBeUndefined()
  })

  it('carries the icon name through', () => {
    const [group] = toNavGroups(
      [{ items: [{ label: 'X', href: '/x', icon: 'search' }] }],
      RouterLink
    )
    expect(group.items[0].icon).toBe('search')
  })

  it('keeps an unlabelled group distinct from a labelled one', () => {
    const mixed: SidebarGroup[] = [{ items: [] }, { group: 'Work', items: [] }, { items: [] }]
    const ids = toNavGroups(mixed, RouterLink).map((g) => g.id)
    expect(new Set(ids).size).toBe(3)
  })

  it('drops an entry with an empty href, which would match every route', () => {
    // The server sends `href: ""` for a nav entry whose kind it does not
    // recognise — rela's own tickets project has a `graph: true` entry and
    // `NavigationEntry` has no `graph` field. Rendered as a link, `to: ''`
    // resolves against the CURRENT location, so the row matches whatever page
    // the user is on and steals the active highlight from the correct row, on
    // every route including ones with no nav entry at all.
    //
    // Found in a browser, not here: it typechecks, renders, and looks like an
    // ordinary row.
    const malformed: SidebarGroup[] = [
      {
        items: [
          { label: 'Graph', href: '' },
          { label: 'Bugs', href: '/list/bugs' },
        ],
      },
    ]
    const [group] = toNavGroups(malformed, RouterLink)
    expect(group.items.map((i) => i.label)).toEqual(['Bugs'])
  })

  it('keeps an action with no href, which is navigable by other means', () => {
    // The same empty href is CORRECT for an action: the server documents that
    // it leaves Href empty so the frontend renders a button. Dropping those
    // would remove every action from the sidebar.
    const [group] = toNavGroups([{ items: [{ label: 'Sync', action: 'sync' }] }], RouterLink)
    expect(group.items.map((i) => i.label)).toEqual(['Sync'])
  })

  it('passes the group heading through as the label', () => {
    const [labelled] = toNavGroups(groups, RouterLink)
    expect(labelled.label).toBe('Work')
    // Undefined rather than empty: the library omits the heading entirely.
    expect(toNavGroups([{ items: [] }], RouterLink)[0].label).toBeUndefined()
  })
})

describe('activeNavId', () => {
  const nav = toNavGroups(
    [
      {
        items: [
          { label: 'Home', href: '/' },
          { label: 'Features', href: '/list/features' },
          { label: 'Sync', action: 'sync' },
        ],
      },
    ],
    RouterLink
  )

  it('matches the row for the current path', () => {
    expect(activeNavId(nav, '/list/features')).toBe('href:/list/features')
  })

  it('keeps a list row active on a detail page beneath it', () => {
    expect(activeNavId(nav, '/list/features/TKT-1')).toBe('href:/list/features')
  })

  it('prefers the longest match, so root does not win everything', () => {
    // `/` is a prefix of every path. A first-match or shortest-match rule would
    // light up Home on every page, which is the bug this ordering prevents.
    expect(activeNavId(nav, '/list/features')).not.toBe('href:/')
    expect(activeNavId(nav, '/')).toBe('href:/')
  })

  it('does not match a path that merely shares a prefix string', () => {
    // `/list/features-archive` starts with `/list/features` as TEXT but is a
    // different route, so a bare startsWith would wrongly highlight it.
    expect(activeNavId(nav, '/list/features-archive')).toBeUndefined()
  })

  it('is undefined on a route with no row, leaving nothing highlighted', () => {
    expect(activeNavId(nav, '/settings')).toBeUndefined()
  })

  it('never matches an action row, which has no target', () => {
    const actions = toNavGroups([{ items: [{ label: 'Sync', action: 'sync' }] }], RouterLink)
    expect(activeNavId(actions, '/sync')).toBeUndefined()
  })
})

describe('toNavGroups flyout entries', () => {
  const groups: SidebarGroup[] = [
    {
      items: [
        { label: 'Mine', href: '/list/mine', flyout: { list: 'mine' } },
        { label: 'All', href: '/list/all' },
      ],
    },
  ]

  it('marks a flyout entry and keeps it a link to the list page', () => {
    const [mine, all] = toNavGroups(groups, RouterLink, () => {})[0].items
    expect(mine.opensFlyout).toBe(true)
    expect(mine.attrs?.to).toBe('/list/mine')
    expect(all.opensFlyout).toBeUndefined()
  })

  it('hands the click to the handler with the row id', () => {
    const calls: [string, string][] = []
    const [mine] = toNavGroups(groups, RouterLink, (item, id) => calls.push([item.label, id]))[0]
      .items
    const onClickCapture = mine.attrs?.onClickCapture as (event: MouseEvent) => void
    onClickCapture(new MouseEvent('click'))
    expect(calls).toEqual([['Mine', mine.id]])
  })

  it('renders a plain link when no handler is given', () => {
    const [mine] = toNavGroups(groups, RouterLink)[0].items
    expect(mine.opensFlyout).toBeUndefined()
    expect(mine.attrs?.onClickCapture).toBeUndefined()
  })
})

describe('toNavGroups status markers', () => {
  const groups: SidebarGroup[] = [
    {
      items: [
        { label: 'Mijn taken', href: '/list/mijn_taken', status_key: '0.0' },
        { label: 'Taken', href: '/list/taken', status_key: '0.1' },
        { label: 'Topics', href: '/list/topics' },
      ],
    },
  ]
  const overdue = { tone: 'error' as const, label: '3 te laat', count: 3 }

  it('attaches the marker the lookup returns for the entry’s key', () => {
    const items = toNavGroups(groups, RouterLink, undefined, (key) =>
      key === '0.0' ? overdue : undefined
    )[0].items
    expect(items[0].status).toEqual(overdue)
  })

  it('leaves an entry with nothing to flag, or no key, without a marker', () => {
    const items = toNavGroups(groups, RouterLink, undefined, (key) =>
      key === '0.0' ? overdue : undefined
    )[0].items
    expect(items[1]).not.toHaveProperty('status')
    expect(items[2]).not.toHaveProperty('status')
  })

  it('looks each entry up by its key, undefined where it has none', () => {
    const asked: Array<string | undefined> = []
    toNavGroups(groups, RouterLink, undefined, (key) => {
      asked.push(key)
      return undefined
    })
    expect(asked).toEqual(['0.0', '0.1', undefined])
  })
})

describe('expandGeneratedItems', () => {
  const topics: SidebarGroup = {
    group: 'Topics',
    items: [],
    items_key: 'pm:1',
    items_page: 'topic',
    items_list: 'actieve_topics',
  }
  const lists: Record<string, NavItemList> = {
    'pm:1': {
      entries: [
        { id: 'TOP-1', type: 'topic', label: 'Website', initial: 'J' },
        { id: 'TOP-2', type: 'topic', label: 'Intranet' },
      ],
    },
  }
  const lookup = (key: string) => lists[key]

  it('fills a generated group with one entry per row, linked to the entity page', () => {
    const [group] = expandGeneratedItems([topics], lookup)
    expect(group.items).toEqual([
      { label: 'Website', href: '/p/topic/TOP-1', initial: 'J' },
      { label: 'Intranet', href: '/p/topic/TOP-2' },
    ])
  })

  it('opens the entity itself when the group names no page', () => {
    const [group] = expandGeneratedItems([{ ...topics, items_page: undefined }], lookup)
    expect(group.items[0].href).toBe('/entity/topic/TOP-1')
  })

  it('leaves static groups alone and drops a generated group with nothing to show', () => {
    const work: SidebarGroup = { group: 'Work', items: [{ label: 'Tasks', href: '/list/tasks' }] }
    expect(expandGeneratedItems([work, { ...topics, items_key: 'pm:9' }], lookup)).toEqual([work])
    expect(expandGeneratedItems([topics], () => ({ entries: [] }))).toEqual([])
  })

  it('keeps an empty generated group that offers a create, with its add label', () => {
    const offer = { type: 'topic', label: 'Topic', form: 'create_topic' }
    const groups = expandGeneratedItems([{ ...topics, items_create: offer }], () => ({ entries: [] }))
    expect(groups).toHaveLength(1)
    expect(groups[0].items).toEqual([])
    const [nav] = toNavGroups(groups, RouterLink)
    expect(nav.addLabel).toBe('New Topic')
  })

  it('gives a group without a create offer no add label', () => {
    const [nav] = toNavGroups(expandGeneratedItems([topics], lookup), RouterLink)
    expect(nav).not.toHaveProperty('addLabel')
  })

  it('links to the whole list when the group is truncated', () => {
    const [group] = expandGeneratedItems([topics], () => ({ ...lists['pm:1'], truncated: true }))
    expect(group.items[group.items.length - 1]).toEqual({
      label: 'Show all',
      href: '/list/actieve_topics',
      icon: 'list',
    })
  })

  it('gives entries distinct ids from their targets, and passes the initial on', () => {
    const [group] = toNavGroups(expandGeneratedItems([topics], lookup), RouterLink)
    expect(group.items.map((i) => i.id)).toEqual(['href:/p/topic/TOP-1', 'href:/p/topic/TOP-2'])
    expect(group.items[0].initial).toBe('J')
    expect(group.items[1].initial).toBeUndefined()
  })

  it('highlights the entry on every tab of its page, and no other entry', () => {
    const nav = toNavGroups(expandGeneratedItems([topics], lookup), RouterLink)
    expect(activeNavId(nav, '/p/topic/TOP-1')).toBe('href:/p/topic/TOP-1')
    expect(activeNavId(nav, '/p/topic/TOP-1/board')).toBe('href:/p/topic/TOP-1')
    expect(activeNavId(nav, '/p/topic/TOP-2/tabel')).toBe('href:/p/topic/TOP-2')
    expect(activeNavId(nav, '/p/topic/TOP-10/board')).toBeUndefined()
  })

  it('escapes an id in the href', () => {
    const [group] = expandGeneratedItems([topics], () => ({
      entries: [{ id: 'A B', type: 'topic', label: 'x' }],
    }))
    expect(group.items[0].href).toBe('/p/topic/A%20B')
  })
})
