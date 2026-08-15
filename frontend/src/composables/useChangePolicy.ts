import type { Bindings } from '@/utils/conditions'
import type { ClearWhenHidden } from '@/types'
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
 * ## The three policies
 *
 * `no` and `yes` decide from state the caller already holds. `confirm` awaits
 * the user, and is the reason the ordering above is not merely tidy: because
 * nothing is mutated and nothing queued before the decision, a decline is a
 * true NO-OP rather than a rollback. Every earlier attempt at `confirm` had to
 * undo a write that had already happened — and each one passed its tests and
 * then failed in manual use.
 *
 * `confirm` is not `yes` with a prompt: declining also abandons the TRIGGERING
 * change, which `yes` never does.
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
  /** Per-property `clear_when_hidden`. */
  policyFor: (property: string) => ClearWhenHidden
  /**
   * Ask the user to approve clearing `properties`. Resolves true to proceed.
   *
   * Only called when something is genuinely at stake — a `confirm` field whose
   * value is non-empty. One dialog per proposal, naming every affected field,
   * never one dialog per field.
   */
  askToClear: (properties: string[]) => Promise<boolean>
  /** True when a property currently holds nothing worth warning about. */
  isEmpty: (property: string) => boolean
  /**
   * Monotonic counter identifying the current form incarnation. Read before
   * awaiting and compared after: if it moved (entity reloaded, form switched
   * entity), the dialog's answer is stale and the proposal is `superseded`.
   */
  generation: () => number
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

  /**
   * Fields this proposal would hide that are configured `confirm` AND actually
   * hold something to lose. An empty field is not worth a dialog — prompting
   * for it trains the user to dismiss without reading.
   */
  function needsConfirm(hiding: string[]): string[] {
    return hiding.filter((p) => deps.policyFor(p) === 'confirm' && !deps.isEmpty(p))
  }

  async function propose(
    property: string,
    value: unknown,
    previous: unknown
  ): Promise<ProposalOutcome> {
    const proposal: Proposal = { property, value, previous }

    // 1. Ask what this would hide. Costs nothing, changes nothing.
    const hiding = hidesFrom(proposal)

    // 2. Decide.
    //
    // Nothing has been mutated and nothing queued at this point, which is the
    // whole reason the seam exists: a decline below is a true no-op, not a
    // rollback. The four BUG-FB0LN8 attempts all had to undo a write here.
    const atStake = needsConfirm(hiding)
    if (atStake.length) {
      const generation = deps.generation()
      const approved = await deps.askToClear(atStake)

      // The form moved on while the dialog was open (entity reloaded, or the
      // form switched entity). `previous` and `hiding` both describe a state
      // that no longer exists, so applying either answer would write against
      // stale assumptions.
      if (deps.generation() !== generation) return { status: 'superseded' }
      if (!approved) return { status: 'rejected' }
    }

    // 3. Apply — retention first, see retainBeforeChange.
    if (hiding.length) retainBeforeChange(hiding, proposal)
    deps.apply(proposal)
    if (hiding.length) deps.onHidden(hiding)

    return { status: 'applied' }
  }

  return { propose, hidesFrom, needsConfirm }
}
