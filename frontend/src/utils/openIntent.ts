// Predicate for "this click is not ours to handle — let the browser have it".
//
// Used by the ONE surface that must keep a click handler alongside a real link:
// the list table row. A `<tr>` cannot be (or contain, at the row level) an
// `<a>`, so the row keeps `@click` for plain navigation while a stretched
// anchor in the title cell provides the link affordance. Everywhere else in the
// SPA the element IS a RouterLink, which applies vue-router's own `guardEvent`
// and needs nothing from here.
//
// Deliberately NOT named `wantsNewTab`: `defaultPrevented` means a nested
// control already handled the event and NOTHING should happen — the opposite of
// "open a tab". Callers use this only to decide whether to skip their own
// `router.push`, never to infer that a tab was opened.
//
// Mirrors vue-router's `guardEvent` and `createDocumentClickHandler` in
// composables/useDocumentClicks.ts, which applies the same rule to links inside
// rendered documents.
export function shouldDeferToBrowser(event: MouseEvent): boolean {
  return (
    event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey
  )
}

// safeInternalHref reports whether a resolved target is a same-origin path we
// are willing to put in an `href`.
//
// Defence-in-depth, not threat mitigation (CLAUDE.md asks which is meant):
// `resolveLinkTarget` — both the Go copy in internal/dataentry/views_handler.go
// and the TS mirror in EntityList.vue — is a closed allowlist over `detail` and
// `document/*`, so a scheme-bearing value cannot reach here today. This exists
// so that if that allowlist is ever loosened, the render layer does not
// silently become an XSS sink.
//
// Requires exactly one leading slash: `//evil.com` is protocol-relative and
// navigates off-origin, so a bare `startsWith('/')` is not sufficient.
export function safeInternalHref(path: string): boolean {
  return /^\/(?!\/)/.test(path)
}
