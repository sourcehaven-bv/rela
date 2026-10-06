/**
 * How much of its scroll pane a sticky bar covers: the bar's height while it
 * is `position: sticky`, and 0 while it sits in normal flow.
 *
 * A bar that sticks on a phone and flows on a desktop covers the top of the
 * pane only in the first case, so anything else that sticks below it needs
 * this number rather than a fixed offset. Measured, not assumed: the bar's
 * height follows its content, the safe-area inset and the touch tap target.
 */
import { onScopeDispose, ref, watch, type Ref } from 'vue'

export function useStickyHeight(el: Ref<HTMLElement | null>): Ref<number> {
  const height = ref(0)
  let observer: ResizeObserver | undefined

  function measure() {
    const bar = el.value
    height.value =
      bar && getComputedStyle(bar).position === 'sticky' ? bar.getBoundingClientRect().height : 0
  }

  watch(
    el,
    (bar) => {
      observer?.disconnect()
      if (bar && typeof ResizeObserver !== 'undefined') {
        observer = new ResizeObserver(measure)
        observer.observe(bar)
      }
      measure()
    },
    { immediate: true, flush: 'post' }
  )

  // Crossing the breakpoint can switch the bar between sticky and in-flow
  // without changing its size, which the observer alone would miss.
  window.addEventListener('resize', measure)
  onScopeDispose(() => {
    observer?.disconnect()
    window.removeEventListener('resize', measure)
  })

  return height
}
