/**
 * Leaving quick entry (TKT-24L1H36). In the desktop popup (`?popup=1`) the
 * window closes. Elsewhere the router goes back, or to the start page when
 * /quick was the first page loaded.
 */
import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'

interface WailsWindow {
  wails?: { Window?: { Close?: () => void; SetSize?: (w: number, h: number) => Promise<void> } }
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

/** Popup width; matches quickEntryWidth in cmd/rela-desktop/quickentry.go. */
export const QUICK_ENTRY_POPUP_WIDTH = 480
/** Tallest popup; a taller form scrolls inside the window. */
export const QUICK_ENTRY_POPUP_MAX_HEIGHT = 720

/**
 * Keeps the desktop popup window as tall as el's content, up to the maximum.
 * Returns a stop function; a no-op outside the desktop app.
 */
export function fitQuickEntryPopup(el: HTMLElement): () => void {
  const setSize = (window as unknown as WailsWindow).wails?.Window?.SetSize
  if (!setSize || typeof ResizeObserver === 'undefined') return () => {}
  let last = 0
  const fit = () => {
    const h = Math.min(Math.ceil(el.scrollHeight), QUICK_ENTRY_POPUP_MAX_HEIGHT)
    if (h === last || h === 0) return
    last = h
    void setSize(QUICK_ENTRY_POPUP_WIDTH, h).catch(() => {})
  }
  const ro = new ResizeObserver(fit)
  ro.observe(el)
  fit()
  return () => ro.disconnect()
}
