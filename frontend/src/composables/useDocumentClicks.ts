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
 *      fragment pointing at the nearest ancestor element that has an
 *      id, so after the form submits vue-router's scrollBehavior scrolls
 *      back near where the user clicked.
 *
 * The enrichment relies on goldmark's auto-generated heading ids —
 * whatever id goldmark emitted is fine; we don't need to know its
 * shape. Elements without a usable ancestor id (e.g. an isolated table
 * cell outside any headed section) get no fragment and land at the top
 * after submit; that's an acceptable fallback.
 *
 * External links (https://, mailto:, tel:, anchor-only) pass through
 * to default browser behaviour.
 */
export function createDocumentClickHandler(router: Router) {
  return (event: MouseEvent) => {
    // Leave modifier-clicks, middle-clicks, and explicitly-targeted links
    // to the browser so the user can open new tabs / new windows / force
    // a real navigation. Only primary-button clicks without any modifier
    // (and without target="_blank" or download) should route through
    // vue-router.
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

    // Find the nearest scroll anchor and splice it into return_to. Use
    // URL-parsing so we don't mis-escape existing query values or step
    // on an existing hash inside return_to.
    const url = new URL(href, window.location.origin)
    const ret = url.searchParams.get('return_to')
    if (ret && !ret.includes('#')) {
      const anchorId = nearestAnchorId(anchor)
      if (anchorId) {
        url.searchParams.set('return_to', ret + '#' + anchorId)
      }
    }

    router.push(url.pathname + url.search)
  }
}

/**
 * Find the id of the nearest heading (or any element with an id) at or
 * before the given element in document order.
 *
 * Walks two ways:
 *
 *   1. Up the ancestor chain, looking for a headed subtree (useful when
 *      the click is inside a heading itself, e.g. the `+` button).
 *
 *   2. If no ancestor has an id, walks *backwards* through the document
 *      tree to find the previous element with an id. This catches the
 *      table-row case: a link inside <tr> isn't inside the <h2> that
 *      precedes the table, but the previous [id] element is that <h2>.
 *
 * Returns `''` when no suitable anchor is found.
 */
function nearestAnchorId(start: Element): string {
  // Ancestor walk first — cheapest and matches the common case
  // (link inside a heading, or inside a <section id="...">).
  const ancestor = start.closest('[id]') as HTMLElement | null
  if (ancestor?.id && !isGlobalChrome(ancestor)) {
    return ancestor.id
  }

  // Backwards-in-document walk for the table-row / paragraph case.
  // Enumerate all in-doc [id] elements and pick the last one at or
  // before `start` in document order. Scoped to the nearest
  // .document-body so global chrome ids (e.g. #app) don't match.
  //
  // If another .document-body is nested inside this one, ids inside
  // it belong to *its* content and shouldn't leak out — reject any
  // candidate whose nearest .document-body isn't our container.
  const container = start.closest('.document-body')
  if (!container) return ''
  const candidates = Array.from(container.querySelectorAll<HTMLElement>('[id]'))
  let best = ''
  for (const el of candidates) {
    if (el.closest('.document-body') !== container) continue
    if (el === start || el.contains(start)) {
      best = el.id
      continue
    }
    const pos = el.compareDocumentPosition(start)
    if (pos & Node.DOCUMENT_POSITION_FOLLOWING) {
      // el precedes start in doc order.
      best = el.id
    } else {
      // el is at or after start — candidates are in doc order, so stop.
      break
    }
  }
  return best
}

// Treat id-bearing elements outside the rendered doc (app shell, side
// panels) as non-anchors — they'd produce a meaningless scroll target.
// Used by the ancestor walk; the preceding-id walk is already scoped
// to the .document-body container.
function isGlobalChrome(el: Element): boolean {
  return !el.closest('.document-body')
}
