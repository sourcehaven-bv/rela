/**
 * Leaving quick entry (TKT-24L1H36). In the desktop popup (`?popup=1`) the
 * window closes. Elsewhere the router goes back, or to the start page when
 * /quick was the first page loaded.
 */
import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'

interface WailsWindow {
  wails?: { Window?: { Close?: () => void } }
}

/** Closes the desktop popup. Returns false when this is not the popup. */
export function closeQuickEntryPopup(route: RouteLocationNormalizedLoaded): boolean {
  const close = (window as unknown as WailsWindow).wails?.Window?.Close
  if (route.query.popup !== '1' || !close) return false
  close()
  return true
}

export function leaveQuickEntry(router: Router, route: RouteLocationNormalizedLoaded): void {
  if (closeQuickEntryPopup(route)) return
  if (window.history.state?.back) router.back()
  else router.push('/')
}

/** True on the desktop quick-entry popup, which App.vue renders without the shell. */
export function isQuickEntryPopup(
  route: Pick<RouteLocationNormalizedLoaded, 'name' | 'query'>
): boolean {
  return route.name === 'quick' && route.query.popup === '1'
}
