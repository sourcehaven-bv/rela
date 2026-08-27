// Timings for the three-indicator pending model (TKT-TFSNBY).
//
// THE ORDERING PRINCIPLE: delay scales with how INVASIVE the indicator is,
// not with how slow the request is. A peripheral bar that fades in costs
// the user almost nothing, so it can appear early. A button label mutating
// under the cursor they just clicked is foveal and disruptive, so it waits
// much longer. This is why published figures like Spectrum's and Primer's
// 1000ms exist at all — they are calibrated for a *spinner on a button*,
// the most invasive combination, and are wrong applied to a top bar.
//
// THE OPERATING MODEL: rela normally talks to a local or well-connected
// server, so the common case is well under 100ms and the occasional slow
// case is 1-2s. Anything past that is a broken connection — an error, not
// a loading state. That is why these are far below the public-web defaults:
// delay + minDuration must stay comfortably under the slow case, or the
// indicator routinely outlives the work it describes.
//
// The governing rule these produce: if an operation completes before its
// indicator's delay elapses, nothing is ever shown. Under the model above
// that means the app shows no loading UI at all in the common case.
//
// These are TUNED BY FEEL, not derived. They are collected here (and
// mirrored as CSS custom properties in styles/scales.css) so adjusting one
// is a single edit rather than a hunt through components.
//
// The ambient/autosave class is deliberately absent: it lives in
// useAutoSave (MIN_SAVING_VISIBLE_MS / SAVED_INDICATOR_MS) and has no entry
// delay of its own, because its 800ms debounce already IS its delay — the
// request does not exist until the user has been quiet that long. See
// RR-ZT9DXG; do not "harmonise" it by adding a delay here.

export const PENDING_TIMINGS = {
  /** Navigation bar — least invasive (peripheral, fades, no reflow). */
  navDelayMs: 250,
  navMinDurationMs: 300,
  /** Button label swap — foveal, at the cursor, so a longer gate. */
  actionDelayMs: 500,
  actionMinDurationMs: 400,
} as const
