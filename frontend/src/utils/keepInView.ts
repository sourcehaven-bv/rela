/**
 * Scrolls an element to the top of its scroll container and keeps it there
 * while the page around it settles (BUG-UZGMHIK). The router uses it for
 * `#anchor` links as well.
 *
 * A single `scrollIntoView` lands only while nothing above the element
 * changes height. On an entity page the view, the document panel and the
 * other panels render after the target does, so the target drifted down to
 * the bottom edge of the pane. This re-scrolls on every animation frame in
 * which the target moved or a new target appeared. It stops once
 * `timeoutMs` has passed and the target has not moved for `idleMs`, after
 * `maxMs` in any case, or as soon as the user scrolls, types or touches the
 * page, or `abort` returns true.
 *
 * Only input events count as the user taking over, not scroll events: a
 * layout shift or the browser's scroll anchoring also scrolls, and reading
 * that as the user would stop the loop in exactly the case it exists for.
 *
 * `target` is read on every frame, so a target that renders later (a step
 * form that waits for its values) takes over from the one found first.
 * Scroll margins (`scroll-margin-top`) apply, so a sticky bar does not cover
 * the target. Returns a function that stops the loop.
 */
export function keepInView(
  target: () => HTMLElement | null | undefined,
  opts: { timeoutMs?: number; idleMs?: number; maxMs?: number; abort?: () => boolean } = {}
): () => void {
  const start = Date.now()
  const minEnd = start + (opts.timeoutMs ?? 3000)
  const maxEnd = start + Math.max(opts.maxMs ?? 8000, opts.timeoutMs ?? 3000)
  const idleMs = opts.idleMs ?? 1000
  let lastMove = start
  let stopped = false
  let frame = 0
  let last: HTMLElement | null = null
  let lastTop = 0

  const scroll = (el: HTMLElement) => {
    el.scrollIntoView?.({ block: 'start' })
    last = el
    lastTop = el.getBoundingClientRect().top
    lastMove = Date.now()
  }

  const tick = () => {
    if (stopped) return
    const now = Date.now()
    if (opts.abort?.() || now > maxEnd || (now > minEnd && now - lastMove > idleMs)) {
      stop()
      return
    }
    const el = target()
    if (el && (el !== last || Math.abs(el.getBoundingClientRect().top - lastTop) > 1)) scroll(el)
    frame = requestAnimationFrame(tick)
  }

  const userTookOver = () => stop()
  const events = ['wheel', 'touchmove', 'keydown', 'pointerdown'] as const

  function stop() {
    if (stopped) return
    stopped = true
    cancelAnimationFrame(frame)
    for (const ev of events) window.removeEventListener(ev, userTookOver)
  }

  for (const ev of events) window.addEventListener(ev, userTookOver, { passive: true })
  const first = target()
  if (first) scroll(first)
  frame = requestAnimationFrame(tick)
  return stop
}
