import { ref } from 'vue'
import type { ClearWhenHidden, FormFieldOrRelation } from '@/types'

/**
 * useHiddenFieldPolicy — what happens to a field's STORED value when its
 * `visible_when` turns false (BUG-FB0LN8).
 *
 * Hiding a field is a presentation decision. It used to be a destructive one:
 * the edit form unset the value server-side the moment the branch hid, and the
 * render gate then refused to draw a field whose key was missing — so the value
 * was gone AND unrecoverable through the UI, from a single dropdown toggle.
 *
 * The policy is per-field `clear_when_hidden`, defaulting to `no`:
 *
 *   no    keep the value. Hide → reveal is lossless: the value is retained
 *         client-side so the reveal needs no server round-trip, and nothing is
 *         written.
 *   yes   clear it when the branch hides.
 *
 * Both are decided synchronously, from state the caller already has. That is
 * deliberate. A third policy — `confirm`, which asks the user first and undoes
 * the triggering change if they decline — is NOT implemented here, because it
 * cannot be done correctly against the current form architecture: an edit
 * mutates `formData` and arms the autosave debounce in one step, so by the time
 * a dialog could resolve, the change is already applied and possibly already
 * sent. Reconstructing the prior state afterwards is what produced a string of
 * near-miss bugs. `confirm` returns when the form separates "the user proposed
 * a change" from "the change was committed"; the backend rejects the value at
 * config-validation time until then.
 *
 * One invariant this composable exists to hold: **retention never lives in
 * `formData`.** It has its own map. Putting retained values back into form
 * state would silently change every existing consumer of `formData` — the
 * autosave disappeared-key sweep would delete them, condition bindings would
 * evaluate against invisible values, and the create-path prune would change
 * meaning.
 */

export interface HiddenFieldPolicyOptions {
  /** Per-property `clear_when_hidden`, defaulting to `no`. */
  policyFor: (property: string) => ClearWhenHidden
}

export function useHiddenFieldPolicy(opts: HiddenFieldPolicyOptions) {
  /**
   * Values of currently-hidden fields, held so a reveal is lossless.
   * Deliberately NOT part of `formData` — see the invariant above.
   */
  const retained = ref<Record<string, unknown>>({})

  /** Remember a hidden field's value so revealing it again is lossless. */
  function retain(property: string, value: unknown): void {
    if (value === undefined) return
    retained.value[property] = value
  }

  /** The retained value for a property, if any. */
  function retainedValue(property: string): unknown {
    return retained.value[property]
  }

  function hasRetained(property: string): boolean {
    return property in retained.value
  }

  /** Drop a retained value — on reveal (consumed) or on clear (destroyed). */
  function release(property: string): void {
    delete retained.value[property]
  }

  /**
   * Drop everything. Called when form state is replaced from the server, since
   * retained values belong to the state being replaced: carrying them across a
   * reload — or across an entity switch, as the form is not re-keyed per
   * entity — would restore one entity's value onto another.
   */
  function releaseAll(): void {
    retained.value = {}
  }

  /**
   * Of the properties about to be hidden, which should be cleared server-side.
   * Everything else is simply retained.
   *
   * `confirm` counts as a clear here because this runs only on the ACCEPTED
   * path: `useChangePolicy` gates the dialog before anything is applied, so by
   * the time this is called the user has already said yes. A declined proposal
   * never reaches it — nothing is applied at all.
   */
  function clearOnHide(hidingProperties: string[]): string[] {
    return hidingProperties.filter((p) => {
      const policy = opts.policyFor(p)
      return policy === 'yes' || policy === 'confirm'
    })
  }

  return {
    retained,
    retain,
    retainedValue,
    hasRetained,
    release,
    releaseAll,
    clearOnHide,
  }
}

/** Read a field's `clear_when_hidden`, defaulting to the non-destructive `no`. */
export function clearWhenHiddenOf(field: FormFieldOrRelation | undefined): ClearWhenHidden {
  return field?.clear_when_hidden ?? 'no'
}
