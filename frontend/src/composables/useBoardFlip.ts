import { getCurrentScope, nextTick, onScopeDispose, watch, type Ref, type WatchSource } from 'vue'

/**
 * Animates cards between renders of a board (FLIP: first, last, invert, play).
 *
 * After every render it records where each card (`[data-item-id]`) sits,
 * relative to the board's scroll content. On the next render a card that
 * moved slides from its old place to its new one, including into another
 * column: the card is matched by id, not by element, so a card Vue re-mounts
 * in a new column still travels. A card that was not there before pops in,
 * one after another on the first render. A column (`[data-section-id]`)
 * that was not there before opens up; the columns of the first render do
 * not, since they are not new to the reader. `flash(id)` marks a card to
 * light up once it has landed.
 *
 * It never moves a card itself; it only animates where the data already put
 * it, so a failed save that leaves the card in place animates nothing.
 * Under reduced motion it only keeps its measurements. After the board's
 * width changes, such as a window resize that reflows the grid, the recorded
 * places are stale; the next render then only measures, so no card slides
 * from where it no longer was.
 */
export function useBoardFlip(
  root: Ref<HTMLElement | undefined>,
  source: WatchSource,
  enabled: () => boolean,
) {
  let last = new Map<string, { x: number; y: number }>()
  let lastSections = new Set<string>()
  let first = true
  let flashId: string | undefined
  let stale = false
  // The animations this board started. Only these are finished before a
  // measurement; a card's own CSS animation, such as the infinite wobble of
  // a drop target, cannot be finished.
  const running = new Set<Animation>()

  function start(node: HTMLElement, keyframes: Keyframe[], options: KeyframeAnimationOptions) {
    const animation = node.animate(keyframes, options)
    running.add(animation)
    const done = () => running.delete(animation)
    animation.addEventListener?.('finish', done)
    animation.addEventListener?.('cancel', done)
  }

  const reduced = () =>
    typeof window.matchMedia !== 'function' ||
    window.matchMedia('(prefers-reduced-motion: reduce)').matches

  function measure(el: HTMLElement): Map<string, { x: number; y: number; node: HTMLElement }> {
    const base = el.getBoundingClientRect()
    const out = new Map<string, { x: number; y: number; node: HTMLElement }>()
    // A running animation would skew the measurement.
    for (const a of [...running]) a.finish()
    running.clear()
    for (const node of el.querySelectorAll<HTMLElement>('[data-item-id]')) {
      const r = node.getBoundingClientRect()
      out.set(node.dataset.itemId as string, {
        x: r.left - base.left + el.scrollLeft,
        y: r.top - base.top + el.scrollTop,
        node,
      })
    }
    return out
  }

  function play() {
    const el = root.value
    if (!el || !enabled()) return
    const now = measure(el)
    const sections = el.querySelectorAll<HTMLElement>('[data-section-id]')
    // jsdom has no Web Animations API; nothing to play there.
    if (!reduced() && !stale && typeof el.animate === 'function') {
      if (!first) {
        for (const node of sections) {
          if (lastSections.has(node.dataset.sectionId as string)) continue
          start(
            node,
            [
              { transform: 'scale(0.85)', opacity: 0 },
              { transform: 'scale(1)', opacity: 1 },
            ],
            { duration: 300, easing: 'cubic-bezier(0.2, 0.8, 0.2, 1)' },
          )
        }
      }
      let stagger = 0
      for (const [id, pos] of now) {
        const before = last.get(id)
        if (before) {
          const dx = before.x - pos.x
          const dy = before.y - pos.y
          if (Math.abs(dx) > 1 || Math.abs(dy) > 1) {
            start(
              pos.node,
              [{ transform: `translate(${dx}px, ${dy}px)` }, { transform: 'translate(0, 0)' }],
              { duration: 320, easing: 'cubic-bezier(0.2, 0.8, 0.2, 1)' },
            )
          }
        } else {
          start(
            pos.node,
            [
              { transform: 'scale(0.6)', opacity: 0 },
              { transform: 'scale(1.04)', opacity: 1, offset: 0.7 },
              { transform: 'scale(1)', opacity: 1 },
            ],
            { duration: 320, easing: 'ease-out', delay: first ? Math.min(stagger, 600) : 0, fill: 'backwards' },
          )
          stagger += 30
        }
        if (id === flashId) {
          start(
            pos.node,
            [
              { boxShadow: '0 0 0 0 var(--rl-color-accent)' },
              { boxShadow: '0 0 0 6px var(--rl-color-accent)', offset: 0.3 },
              { boxShadow: '0 0 0 0 transparent' },
            ],
            { duration: 700, delay: 250, easing: 'ease-out' },
          )
          flashId = undefined
        }
      }
    }
    stale = false
    // The first render with content is the page load; an empty one is not.
    if (now.size || sections.length) first = false
    lastSections = new Set([...sections].map((n) => n.dataset.sectionId as string))
    last = new Map([...now].map(([id, { x, y }]) => [id, { x, y }]))
  }

  watch(source, () => nextTick(play), { flush: 'post', immediate: true })

  // Only the width: the height changes with every card added or moved.
  // jsdom has no ResizeObserver.
  let width: number | undefined
  if (typeof ResizeObserver === 'function') {
    const observer = new ResizeObserver(([entry]) => {
      const now = entry.contentRect.width
      if (width !== undefined && now !== width) stale = true
      width = now
    })
    watch(root, (el, old) => {
      if (old) observer.unobserve(old)
      if (el) observer.observe(el)
    }, { immediate: true })
    if (getCurrentScope()) onScopeDispose(() => observer.disconnect())
  }

  return {
    flash(id: string) {
      flashId = id
    },
  }
}
