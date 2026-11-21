import type { Router } from 'vue-router'
import { spaceOf, withSpace } from '@/stores/space'
import { isConfigurePath } from '@/configure/routes'

/**
 * Makes every link the router resolves carry the current space.
 *
 * Links in the SPA are built as plain paths (`/entity/...`, `/list/...`), and
 * a navigation guard sends such a path to the same path in the current space.
 * The guard runs on navigation only, so a link's `href` stayed unprefixed: the
 * status bar, a copied address and a new tab all got the bare path, and a page
 * opened from it loaded once without the space before the redirect.
 *
 * A router link takes its `href` from `router.resolve`, so prefixing there
 * fixes every link at once without teaching each builder about spaces. A
 * navigation is unaffected: `push` resolves through the router's own closure
 * and still meets the guard. Paths the guard leaves alone (already prefixed,
 * the configure pages) are left alone here too.
 *
 * `space` returns the current space id, or undefined when spaces are off.
 */
export function prefixResolvedLinks(router: Router, space: () => string | undefined): void {
  const resolve = router.resolve.bind(router)
  router.resolve = ((
    to: Parameters<Router['resolve']>[0],
    current?: Parameters<Router['resolve']>[1]
  ) => {
    const resolved = resolve(to, current)
    const id = space()
    const path = resolved.path
    if (!id || spaceOf(path) || isConfigurePath(path) || !path.startsWith('/')) return resolved
    return resolve(
      { path: withSpace(path, id), query: resolved.query, hash: resolved.hash },
      current
    )
  }) as Router['resolve']
}
