import type { Bindings } from '@/utils/conditions'

/**
 * Proposals — "the user wants to change X to Y", as a value, before anything
 * is applied (TKT-7S5735).
 *
 * ## Why this exists
 *
 * `DynamicForm.updateField` writes `formData` on its first line and arms the
 * autosave debounce on its last. There is no point in between where a change
 * exists but is not yet committed, so anything that needs to intervene — a
 * confirmation dialog above all — has to run AFTER the change is applied and
 * possibly already sent, then reconstruct the prior state. BUG-FB0LN8 produced
 * four such reconstructions; each passed its tests and each then failed in
 * manual use.
 *
 * The hard part is not the dialog. It is that **visibility is a pure function
 * of `formData`**: `DynamicForm`'s `conditionBindings` feeds `form:
 * formData.value` into every `visible_when`, so the question "would this edit
 * hide a field?" was only answerable by writing the value first. That is the
 * conflation itself, and no amount of care after the write can undo it.
 *
 * So the answer is computed against a hypothetical binding set instead. Nothing
 * is mutated, nothing is scheduled, and the caller decides afterwards.
 *
 * ## Shape
 *
 * Everything here is pure and synchronous — no Vue reactivity, no component
 * mount, no I/O. That is deliberate: this is precisely where the four
 * BUG-FB0LN8 bugs lived, and it is now the easiest layer in the form to test.
 *
 * The pre-change value travels INSIDE the proposal rather than in a shared
 * mutable slot. A single `lastEdit`-style slot was one of the near-miss bugs:
 * a second watcher pass consumed it, and the revert then restored the wrong
 * value.
 */

/** A change the user wants to make, not yet applied. */
export interface Proposal {
  property: string
  /** The value being proposed. */
  value: unknown
  /** The value before this proposal — the exact restore target on reject. */
  previous: unknown
}

/**
 * The outcome of proposing a change.
 *
 * `superseded` is load-bearing: it is what an entity reload landing mid-dialog
 * (or a form switching entity) must produce. Naming it forces every call site
 * to handle the case instead of discovering it later as a stale-write bug.
 */
export type ProposalOutcome =
  | { status: 'applied' }
  | { status: 'rejected' }
  | { status: 'superseded' }

/**
 * Bindings as they WOULD be if `proposal` were applied.
 *
 * Shallow-copies the `form` namespace so the live object is never touched —
 * the whole point is that asking the question costs nothing.
 */
export function proposedBindings(current: Bindings, proposal: Proposal): Bindings {
  const form = { ...((current.form as Record<string, unknown>) ?? {}) }
  form[proposal.property] = proposal.value
  return { ...current, form }
}

/**
 * Properties that are visible now but would NOT be if `proposal` were applied.
 *
 * Direct hides only — this is a set difference over one proposal, not a
 * transitive closure. A field whose own condition depends on a field this
 * proposal hides may go stale; that limitation is accepted and documented on
 * BUG-FB0LN8 (`clear_when_hidden: yes` is the escape hatch), and widening it
 * here would mean evaluating the form to a fixpoint on every keystroke.
 *
 * `managed` filters to keys the wizard actually governs, so a property that is
 * merely absent from the config (e.g. a metamodel default seeded into form
 * state but surfaced in no step) is never reported as "hidden".
 */
export function wouldHide(
  activeNow: Set<string>,
  activeAfter: Set<string>,
  managed: Set<string>
): string[] {
  const hidden: string[] = []
  for (const property of activeNow) {
    if (activeAfter.has(property)) continue
    if (!managed.has(property)) continue
    hidden.push(property)
  }
  return hidden
}
