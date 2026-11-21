import { describe, it, expect } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'
import { prefixResolvedLinks } from './spaceLinks'
import { spaceOf, withSpace } from '@/utils/spacePath'

const Blank = { template: '<div />' }

function routerIn(space: string | undefined) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/entity/:type/:id', name: 'entity', component: Blank },
      { path: '/s/:space/entity/:type/:id', component: Blank },
      { path: '/configure/schema', component: Blank },
      { path: '/:rest(.*)*', component: Blank },
    ],
  })
  prefixResolvedLinks(router, () => space)
  return router
}

describe('prefixResolvedLinks', () => {
  it('puts the current space in front of an unprefixed link', () => {
    const r = routerIn('portal').resolve('/entity/account/ACC-1?run=RUN-1#step')
    expect(r.path).toBe('/s/portal/entity/account/ACC-1')
    expect(r.query).toEqual({ run: 'RUN-1' })
    expect(r.hash).toBe('#step')
    expect(r.href).toBe('/s/portal/entity/account/ACC-1?run=RUN-1#step')
  })

  it('resolves a location object the same way', () => {
    const r = routerIn('portal').resolve({ path: '/entity/account/ACC-1', query: { world: 'w' } })
    expect(r.fullPath).toBe('/s/portal/entity/account/ACC-1?world=w')
  })

  it('leaves a link that already names a space alone', () => {
    expect(routerIn('portal').resolve('/s/backoffice/entity/account/ACC-1').path).toBe(
      '/s/backoffice/entity/account/ACC-1'
    )
  })

  it('leaves the configure pages alone, like the navigation guard', () => {
    expect(routerIn('portal').resolve('/configure/schema').path).toBe('/configure/schema')
  })

  it('changes nothing without spaces', () => {
    expect(routerIn(undefined).resolve('/entity/account/ACC-1').path).toBe('/entity/account/ACC-1')
  })

  it('prefixes a named route', () => {
    const r = routerIn('portal').resolve({
      name: 'entity',
      params: { type: 'account', id: 'ACC-1' },
    })
    expect(r.path).toBe('/s/portal/entity/account/ACC-1')
  })

  it('prefixes a relative link from a page in the space', async () => {
    const router = routerIn('portal')
    await router.push('/s/portal/entity/account/ACC-1')
    expect(router.resolve('ACC-2').path).toBe('/s/portal/entity/account/ACC-2')
  })

  // The doc comment's claim: push does not go through the wrapper, so a
  // navigation still meets the guard, which is what moves it into the space.
  it('leaves navigation to the guard', async () => {
    const router = routerIn('portal')
    const seen: string[] = []
    router.beforeEach((to) => {
      seen.push(to.path)
      if (!spaceOf(to.path)) return withSpace(to.fullPath, 'portal')
    })
    await router.push('/entity/account/ACC-1')
    expect(seen[0]).toBe('/entity/account/ACC-1')
    expect(router.currentRoute.value.path).toBe('/s/portal/entity/account/ACC-1')
  })
})
