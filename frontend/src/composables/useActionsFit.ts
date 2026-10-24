import { onBeforeUnmount, onMounted, ref, watch, nextTick, type Ref, type WatchSource } from 'vue'

/**
 * Decides whether a header's full action row fits beside its title.
 *
 * `container` holds the title block (`info`) and the action row (`row`). The
 * row fits when its natural width plus the title's CSS `min-width` and the
 * container's column gap is no wider than the container. When it does not
 * fit, `compact` turns true and the caller renders a shorter row. The full
 * row's width is remembered from the last time it was drawn, so the row
 * comes back once the container is wide enough again.
 *
 * `contentKey` changes when the set of actions changes. The full row is then
 * drawn and measured again, because the remembered width no longer applies.
 */
export function useActionsFit(
  container: Ref<HTMLElement | null>,
  info: Ref<HTMLElement | null>,
  row: Ref<HTMLElement | null>,
  contentKey: WatchSource<unknown>
) {
  const compact = ref(false)
  let fullWidth = 0
  let observer: ResizeObserver | null = null

  function px(value: string): number {
    const n = parseFloat(value)
    return Number.isFinite(n) ? n : 0
  }

  function available(): number {
    const c = container.value
    if (!c) return Infinity
    const style = getComputedStyle(c)
    const titleMin = info.value ? px(getComputedStyle(info.value).minWidth) : 0
    return c.clientWidth - titleMin - px(style.columnGap)
  }

  function measure() {
    // A container that is not laid out yet tells nothing; keep the full row.
    if (!container.value?.clientWidth) return
    const r = row.value
    if (!compact.value && r) {
      // A hidden row (the phone layout) measures 0 and always fits.
      fullWidth = r.scrollWidth
    }
    compact.value = fullWidth > available()
  }

  watch(contentKey, async () => {
    compact.value = false
    await nextTick()
    measure()
  })

  onMounted(() => {
    if (typeof ResizeObserver === 'undefined') return
    observer = new ResizeObserver(() => measure())
    if (container.value) observer.observe(container.value)
    // The row grows on its own when a menu loads its entries.
    if (row.value) observer.observe(row.value)
    measure()
  })
  onBeforeUnmount(() => observer?.disconnect())

  return { compact, measure }
}
