// The pre-link contract (TKT-R4BMJM).
//
// These exist because the two surfaces that build a create link disagreed about
// the query-param names and nothing caught it: `SidePanel.vue` wrote
// `_relation` / `_linkAs` / `_peerId`, `DynamicForm.vue` read `link_relation` /
// `link_peer` / `link_as`, and a repo-wide grep found exactly one write site and
// zero read sites for the underscore spelling. The Add button therefore opened a
// create form with no relation context, and the user linked by hand — the chore
// the button exists to remove.
//
// A test asserting the literals it expects would pass against that bug, so the
// real guard is `SidePanel.test.ts`, which asserts the emitted URL against the
// names DynamicForm actually parses. This file covers the builder itself.

import { describe, it, expect } from 'vitest'
import { buildCreateLinkQuery, LINK_QUERY_KEYS } from './createLink'

describe('buildCreateLinkQuery', () => {
  it('emits the three required params under the names the form reads', () => {
    const q = buildCreateLinkQuery({ relation: 'has-task', peer: 'EPIC-1', linkAs: 'to' })

    expect(q).toEqual({
      link_relation: 'has-task',
      link_peer: 'EPIC-1',
      link_as: 'to',
    })
  })

  it('carries linkAs verbatim, since it decides the edge direction', () => {
    // 'from' and 'to' produce opposite edges. A builder that normalised or
    // defaulted this would silently reverse a relation.
    expect(buildCreateLinkQuery({ relation: 'r', peer: 'P-1', linkAs: 'from' }).link_as).toBe('from')
    expect(buildCreateLinkQuery({ relation: 'r', peer: 'P-1', linkAs: 'to' }).link_as).toBe('to')
  })

  it('omits optional params rather than emitting empty values', () => {
    // An empty `return_to` would be read as a return target and fail the
    // open-redirect guard's shape checks; an empty `template` would ask the form
    // to select a variant named "".
    const q = buildCreateLinkQuery({ relation: 'r', peer: 'P-1', linkAs: 'to' })

    expect(q).not.toHaveProperty(LINK_QUERY_KEYS.returnTo)
    expect(q).not.toHaveProperty(LINK_QUERY_KEYS.template)
    expect(q).not.toHaveProperty(LINK_QUERY_KEYS.world)
  })

  it('includes the optional params when given', () => {
    const q = buildCreateLinkQuery({
      relation: 'has-task',
      peer: 'EPIC-1',
      linkAs: 'to',
      returnTo: '/entity/epic/EPIC-1',
      template: 'bugfix',
      world: 'published',
    })

    expect(q.return_to).toBe('/entity/epic/EPIC-1')
    expect(q.template).toBe('bugfix')
    expect(q.world).toBe('published')
  })

  it('never emits an underscore-prefixed key', () => {
    // The exact shape of the bug: these names are written by nothing now, and a
    // reintroduction would mean a button that silently fails to pre-link.
    const q = buildCreateLinkQuery({
      relation: 'r',
      peer: 'P-1',
      linkAs: 'to',
      returnTo: '/x',
      template: 't',
      world: 'w',
    })

    for (const key of Object.keys(q)) {
      expect(key.startsWith('_')).toBe(false)
    }
  })
})
