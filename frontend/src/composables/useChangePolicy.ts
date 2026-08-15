import type { Bindings } from '@/utils/conditions'
import { proposedBindings, wouldHide, type Proposal, type ProposalOutcome } from './useProposal'

/**
 * useChangePolicy — decide the fate of a proposed field edit BEFORE it is
 * applied (TKT-7S5735).
 *
 * ## The seam
 *
 * A widget edit arrives as a proposal, not a write:
 *
 *   1. work out what the change would hide (hypothetical, no mutation)
 *   2. decide
 *   3. only then mutate form state and arm the write
 *
 * Steps 1-2 happen before any mutation, which the old code could not do:
 * `updateField` wrote `formData` on its first line, and since visibility is a
 * pure function of `formData`, "what would hide?" was only answerable after the
 * fact. Every BUG-FB0LN8 fix tried to reconstruct the prior state from that
 * position, and each produced a near-miss.
 *
 * ## What this does NOT own
 *
 * `formData`. The host form applies the outcome. A policy that owned form state
 * would just be a second god-object beside a component already past 2000 lines,
 * and the host has to apply the change anyway (errors, dirty tracking, autosave
 * routing all live there).
 *
 * ## Today's policies are synchronous
 *
 * `no` and `yes` both decide from state the caller already holds, so
 * `propose()` always returns `applied`. The seam exists so an interactive
 * policy (`clear_when_hidden: confirm`) can return `rejected` with not a single
 * byte changed. That policy is not implemented here — it needs the write queue
 * half of TKT-7S5735 — but nothing about this shape has to change to add it.
 */

export interface ChangePolicyDeps {
  /** Live condition bindings (`form`, `entity`, `current_user`). */
  bindings: () => Bindings
  /** Property keys visible right now. */
  activeNow: () => Set<string>
  /** Property keys that would be visible for a hypothetical binding set. */
  activeFor: (bindings: Bindings) => Set<string>
  /** Property keys the wizard governs at all. */
  managed: () => Set<string>
  /** Current form value of a property (for retaining a non-trigger field). */
  valueOf: (property: string) => unknown
  /** Hold a field's value so a later reveal is lossless. */
  retain: (property: string, value: unknown) => void
  /** Commit an accepted proposal to form state + the write queue. */
  apply: (proposal: Proposal) => void
  /** Apply `clear_when_hidden` to fields that just hid (already retained). */
  onHidden: (hiding: string[]) => void
  /**
   * Whether hide policy applies at all. False on the create path, where there
   * is no stored value to lose — RR-O4SRG's drop-on-commit owns that.
   */
  enabled: () => boolean
}

export function useChangePolicy(deps: ChangePolicyDeps) {
  /**
   * Properties that are visible now but would not be if `proposal` applied.
   * Pure: no mutation, no scheduling, safe to call as often as you like.
   */
  function hidesFrom(proposal: Proposal): string[] {
    if (!deps.enabled()) return []
    const after = deps.activeFor(proposedBindings(deps.bindings(), proposal))
    return wouldHide(deps.activeNow(), after, deps.managed())
  }

  /**
   * Retain the PRE-change value of every field this proposal hides.
   *
   * Ordering is what makes this correct: `propose` calls this BEFORE
   * `deps.apply`, so `valueOf` still reads pre-change form state.
   *
   * The proposed property is read from `proposal.previous` rather than
   * `valueOf` as belt-and-braces, not as the fix. It matters only if a field
   * hides ITSELF — `visible_when: "form.mode == 'detail'"` on property `mode` —
   * and only if the retain/apply order above is ever inverted. Since the
   * ordering already protects that case, this line is unobservable today; it is
   * kept so that reordering degrades into a redundant read rather than a silent
   * data bug, which is the failure mode this whole ticket exists to prevent.
   */
  function retainBeforeChange(hiding: string[], proposal: Proposal) {
    for (const property of hiding) {
      deps.retain(
        property,
        property === proposal.property ? proposal.previous : deps.valueOf(property)
      )
    }
  }

  function propose(property: string, value: unknown, previous: unknown): ProposalOutcome {
    const proposal: Proposal = { property, value, previous }

    // 1. Ask. Costs nothing, changes nothing.
    const hiding = hidesFrom(proposal)

    // 2. Decide. Synchronous today; this is where an interactive policy awaits.

    // 3. Apply — retention first, see retainBeforeChange.
    if (hiding.length) retainBeforeChange(hiding, proposal)
    deps.apply(proposal)
    if (hiding.length) deps.onHidden(hiding)

    return { status: 'applied' }
  }

  return { propose, hidesFrom }
}
