import { onBeforeUnmount, onMounted } from 'vue'

/**
 * Mirror window.visualViewport.offsetTop onto the --vv-offset-top CSS
 * variable on <html>.
 *
 * iOS WebKit anchors `position: sticky` to the layout viewport, not the
 * visual viewport. When the keyboard opens, the visual viewport shifts
 * up by ~68px but a sticky `top: 0` header stays glued to the (now
 * off-screen) layout-viewport top, sliding under the status bar. Sticky
 * topbars consume this var via translateY() to follow the visual
 * viewport.
 *
 * Intended to be called once from App.vue. Listeners are removed on
 * unmount so HMR and tests don't leak handlers.
 */
export function useVisualViewportOffset() {
  let vv: VisualViewport | null = null
  let sync: (() => void) | null = null

  onMounted(() => {
    if (typeof window === 'undefined') return
    vv = window.visualViewport
    if (!vv) return
    sync = () => {
      document.documentElement.style.setProperty('--vv-offset-top', `${vv!.offsetTop}px`)
    }
    sync()
    vv.addEventListener('resize', sync)
    vv.addEventListener('scroll', sync)
  })

  onBeforeUnmount(() => {
    if (vv && sync) {
      vv.removeEventListener('resize', sync)
      vv.removeEventListener('scroll', sync)
    }
    vv = null
    sync = null
  })
}
