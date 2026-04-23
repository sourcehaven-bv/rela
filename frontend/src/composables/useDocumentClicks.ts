import { type Router } from 'vue-router'

/**
 * Click handler for links inside a rendered document's HTML.
 *
 * Two jobs:
 *
 *   1. Intercept internal links (starting with `/`) and route them
 *      through vue-router instead of triggering a full-page reload.
 *
 *   2. Enrich any `return_to` query param on the href with a `#<id>`
 *      fragment taken from the clicked anchor's own id, so after the
 *      form submits vue-router's scrollBehavior scrolls back to the
 *      exact link the user clicked. The server emits a stable id on
 *      every edit/create form link (e.g. id="edit-prs-bf-7hn6-0" or
 *      id="create-businessfunctie-0") derived from the link's URL path
 *      — so the id survives title and content edits.
 *
 * External links (https://, mailto:, tel:, anchor-only) pass through
 * to default browser behaviour.
 *
 * Modifier-clicks (cmd/ctrl/shift/alt), middle/right-clicks, and links
 * with target="_blank" or download also pass through — the user asked
 * for a new tab / window / download, don't steal the intent.
 */
export function createDocumentClickHandler(router: Router) {
  return (event: MouseEvent) => {
    if (
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return
    }

    const target = event.target as HTMLElement
    const anchor = target.closest('a') as HTMLAnchorElement | null
    if (!anchor) return
    if (anchor.target && anchor.target !== '' && anchor.target !== '_self') return
    if (anchor.hasAttribute('download')) return

    const href = anchor.getAttribute('href')
    if (!href || !href.startsWith('/')) return // external / anchor / mailto

    event.preventDefault()

    // If the rewriter injected a return_to and the link has a stable id,
    // append that id as the #fragment so the submit redirect scrolls
    // back exactly here. URL-parse so we don't mis-escape existing query
    // values or step on an existing hash.
    const url = new URL(href, window.location.origin)
    const ret = url.searchParams.get('return_to')
    if (ret && !ret.includes('#') && anchor.id) {
      url.searchParams.set('return_to', ret + '#' + anchor.id)
    }

    router.push(url.pathname + url.search)
  }
}
