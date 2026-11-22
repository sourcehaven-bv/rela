import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue'

/**
 * The widest viewport that counts as a phone. It matches the breakpoint at
 * which RlSidebar turns into a drawer (`max-width: 767px`), so "on a phone"
 * means the same screens everywhere a setting refers to it.
 */
export const PHONE_MEDIA_QUERY = '(max-width: 767px)'

/**
 * Whether the viewport is phone-sized, kept up to date as it resizes or
 * rotates. False where the browser has no matchMedia (as in some tests),
 * which keeps the wide-screen behaviour.
 */
export function usePhoneScreen(): Ref<boolean> {
  const hasMatchMedia = typeof window !== 'undefined' && typeof window.matchMedia === 'function'
  // Read at once, so the first render on a phone already leaves the rows out.
  const phone = ref(hasMatchMedia && window.matchMedia(PHONE_MEDIA_QUERY).matches)
  let mql: MediaQueryList | null = null
  const sync = (e: MediaQueryList | MediaQueryListEvent) => {
    phone.value = e.matches
  }

  onMounted(() => {
    if (!hasMatchMedia) return
    mql = window.matchMedia(PHONE_MEDIA_QUERY)
    sync(mql)
    mql.addEventListener('change', sync)
  })

  onBeforeUnmount(() => {
    mql?.removeEventListener('change', sync)
    mql = null
  })

  return phone
}
