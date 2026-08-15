// Unit tests for the change policy (TKT-7S5735).
//
// The retention ORDER is the subtle part and the reason these exist. Two
// mutations survived the component-level harness — removing retention
// entirely, and retaining the post-change value — because under the default
// `clear_when_hidden: no` policy nothing is deleted from `formData`, so a
// reveal appears lossless whether or not retention ever ran. These tests pin
// the mechanism directly instead of through its most forgiving consumer.

import { describe, it, expect, vi } from 'vitest'
import { useChangePolicy, type ChangePolicyDeps } from './useChangePolicy'
import type { Proposal } from './useProposal'
import type { Bindings } from '@/utils/conditions'

/**
 * A policy wired to a plain object form, with visibility driven by an explicit
 * rule so a test can express "this edit hides that field" without a metamodel.
 */
function setup(opts: {
  form: Record<string, unknown>
  /** Which properties are visible for a given form state. */
  visibleFor: (form: Record<string, unknown>) => string[]
  managed?: string[]
  enabled?: boolean
  policyFor?: (property: string) => string
  /** What the confirm dialog answers. */
  approve?: boolean
}) {
  const form = { ...opts.form }
  const retained: Record<string, unknown> = {}
  const asked: string[][] = []
  const generation = { value: 0 }
  const applied: Proposal[] = []
  const hiddenBatches: string[][] = []

  const deps: ChangePolicyDeps = {
    bindings: (): Bindings => ({ form, entity: {}, current_user: {} }),
    activeNow: () => new Set(opts.visibleFor(form)),
    activeFor: (b) => new Set(opts.visibleFor((b.form ?? {}) as Record<string, unknown>)),
    managed: () => new Set(opts.managed ?? Object.keys(opts.form)),
    valueOf: (p) => form[p],
    retain: (p, v) => {
      retained[p] = v
    },
    apply: (proposal) => {
      applied.push(proposal)
      form[proposal.property] = proposal.value
    },
    onHidden: (hiding) => hiddenBatches.push(hiding),
    enabled: () => opts.enabled ?? true,
    policyFor: (p) => (opts.policyFor?.(p) ?? 'no') as never,
    askToClear: async (props) => {
      asked.push(props)
      return opts.approve ?? true
    },
    isEmpty: (p) => form[p] === undefined || form[p] === '' || form[p] === null,
    generation: () => generation.value,
  }

  return { policy: useChangePolicy(deps), form, retained, applied, hiddenBatches, asked, generation }
}

// The reporter's shape: deadlines visible only while route == 'aanbesteding'.
const routeRule = (form: Record<string, unknown>) =>
  form.route === 'aanbesteding' ? ['route', 'deadline'] : ['route']

describe('useChangePolicy', () => {
  it('applies an accepted proposal', async () => {
    const { policy, form, applied } = setup({
      form: { route: 'aanbesteding', deadline: '2026-09-15' },
      visibleFor: routeRule,
    })
    expect(await policy.propose('route', 'onderhands', 'aanbesteding')).toEqual({ status: 'applied' })
    expect(form.route).toBe('onderhands')
    expect(applied).toHaveLength(1)
  })

  it('reports the fields a proposal would hide', async () => {
    const { policy, hiddenBatches } = setup({
      form: { route: 'aanbesteding', deadline: '2026-09-15' },
      visibleFor: routeRule,
    })
    await policy.propose('route', 'onderhands', 'aanbesteding')
    expect(hiddenBatches).toEqual([['deadline']])
  })

  it('reports nothing hidden when visibility is unchanged', async () => {
    const { policy, hiddenBatches } = setup({
      form: { route: 'aanbesteding', deadline: '2026-09-15' },
      visibleFor: routeRule,
    })
    await policy.propose('deadline', '2026-10-01', '2026-09-15')
    expect(hiddenBatches).toEqual([])
  })

  // Pins the retention mechanism itself: it must run, and it must capture the
  // value as it was BEFORE the change.
  it('retains a hidden field s value before applying the change', async () => {
    const { policy, retained } = setup({
      form: { route: 'aanbesteding', deadline: '2026-09-15' },
      visibleFor: routeRule,
    })
    await policy.propose('route', 'onderhands', 'aanbesteding')
    expect(retained).toEqual({ deadline: '2026-09-15' })
  })

  it('retains nothing when the change hides nothing', async () => {
    const { policy, retained } = setup({
      form: { route: 'aanbesteding', deadline: '2026-09-15' },
      visibleFor: routeRule,
    })
    await policy.propose('deadline', '2026-10-01', '2026-09-15')
    expect(retained).toEqual({})
  })

  // A field CAN hide itself: `visible_when: "form.mode == 'detail'"` on
  // property `mode`. Retaining the post-change value would make a later reveal
  // restore what the user was leaving rather than what they had.
  it('retains the PRE-change value when a field hides itself', async () => {
    const selfRule = (form: Record<string, unknown>) => (form.mode === 'detail' ? ['mode'] : [])
    const { policy, retained } = setup({
      form: { mode: 'detail' },
      visibleFor: selfRule,
      managed: ['mode'],
    })

    await policy.propose('mode', 'summary', 'detail')

    expect(retained.mode).toBe('detail') // not 'summary'
  })

  // What actually protects the case above is the ORDER — retention runs before
  // the change is applied, so form state is still pre-change when read. Pin it
  // directly: the self-hiding test alone passes even with the ordering
  // inverted, because the proposal carries `previous` as a second line of
  // defence.
  it('retains before applying, never after', async () => {
    const order: string[] = []
    const policy = useChangePolicy({
      bindings: () => ({ form: { route: 'aanbesteding' }, entity: {}, current_user: {} }),
      activeNow: () => new Set(['route', 'deadline']),
      activeFor: () => new Set(['route']),
      managed: () => new Set(['route', 'deadline']),
      valueOf: () => '2026-09-15',
      retain: () => order.push('retain'),
      apply: () => order.push('apply'),
      onHidden: () => order.push('onHidden'),
      enabled: () => true,
      policyFor: () => 'no',
      askToClear: async () => true,
      isEmpty: () => false,
      generation: () => 0,
    })

    await policy.propose('route', 'onderhands', 'aanbesteding')

    expect(order).toEqual(['retain', 'apply', 'onHidden'])
  })

  it('retains every field in a multi-field hide', async () => {
    const twoRule = (form: Record<string, unknown>) =>
      form.route === 'aanbesteding' ? ['route', 'a', 'b'] : ['route']
    const { policy, retained, hiddenBatches } = setup({
      form: { route: 'aanbesteding', a: 1, b: 2 },
      visibleFor: twoRule,
    })

    await policy.propose('route', 'onderhands', 'aanbesteding')

    expect(retained).toEqual({ a: 1, b: 2 })
    expect(hiddenBatches[0].sort()).toEqual(['a', 'b']) // ONE batch, not one per field
  })

  // The create path has no stored value to lose; RR-O4SRG's drop-on-commit
  // owns it. The policy must not retain or report hides there.
  it('does nothing when disabled (create path)', async () => {
    const { policy, retained, hiddenBatches, form } = setup({
      form: { route: 'aanbesteding', deadline: '2026-09-15' },
      visibleFor: routeRule,
      enabled: false,
    })

    await policy.propose('route', 'onderhands', 'aanbesteding')

    expect(retained).toEqual({})
    expect(hiddenBatches).toEqual([])
    expect(form.route).toBe('onderhands') // the edit itself still applies
  })

  it('ignores an unmanaged property that stops being visible', async () => {
    const { policy, retained, hiddenBatches } = setup({
      form: { route: 'aanbesteding', stray: 'x' },
      visibleFor: (form) => (form.route === 'aanbesteding' ? ['route', 'stray'] : ['route']),
      managed: ['route'], // `stray` is not wizard-governed
    })

    await policy.propose('route', 'onderhands', 'aanbesteding')

    expect(retained).toEqual({})
    expect(hiddenBatches).toEqual([])
  })

  // `confirm` is the policy the whole refactor exists for. It is NOT "yes with
  // a prompt": declining abandons the TRIGGERING change too, which `yes` never
  // does. That is only correct because nothing is mutated or queued before the
  // decision — a decline is a no-op, not a rollback.
  describe('clear_when_hidden: confirm', () => {
    const confirmSetup = (approve: boolean, form?: Record<string, unknown>) =>
      setup({
        form: form ?? { route: 'aanbesteding', deadline: '2026-09-15' },
        visibleFor: routeRule,
        policyFor: (p) => (p === 'deadline' ? 'confirm' : 'no'),
        approve,
      })

    it('asks once, naming every field at stake', async () => {
      const { policy, asked } = confirmSetup(true)
      await policy.propose('route', 'onderhands', 'aanbesteding')
      expect(asked).toEqual([['deadline']])
    })

    it('applies the change and clears when approved', async () => {
      const { policy, form, hiddenBatches } = confirmSetup(true)
      const outcome = await policy.propose('route', 'onderhands', 'aanbesteding')

      expect(outcome).toEqual({ status: 'applied' })
      expect(form.route).toBe('onderhands')
      expect(hiddenBatches).toEqual([['deadline']])
    })

    // The heart of it. On decline NOTHING happens — not the clear, and not the
    // triggering edit either. No rollback is needed because no write occurred.
    it('abandons the triggering change when declined', async () => {
      const { policy, form, applied, retained, hiddenBatches } = confirmSetup(false)
      const outcome = await policy.propose('route', 'onderhands', 'aanbesteding')

      expect(outcome).toEqual({ status: 'rejected' })
      expect(form.route).toBe('aanbesteding') // unchanged, never written
      expect(applied).toEqual([]) // apply was never called
      expect(retained).toEqual({}) // nothing retained either
      expect(hiddenBatches).toEqual([]) // and nothing cleared
    })

    // Prompting about an empty field trains people to dismiss without reading.
    it('does not ask when the hidden field holds nothing', async () => {
      const { policy, asked, form } = confirmSetup(false, {
        route: 'aanbesteding',
        deadline: '',
      })
      const outcome = await policy.propose('route', 'onderhands', 'aanbesteding')

      expect(asked).toEqual([])
      expect(outcome).toEqual({ status: 'applied' }) // no stake → no gate
      expect(form.route).toBe('onderhands')
    })

    it('does not ask for a field whose policy is not confirm', async () => {
      const { policy, asked } = setup({
        form: { route: 'aanbesteding', deadline: '2026-09-15' },
        visibleFor: routeRule,
        policyFor: () => 'yes',
      })
      await policy.propose('route', 'onderhands', 'aanbesteding')
      expect(asked).toEqual([])
    })

    // An entity reload (or an entity switch) while the dialog is open means
    // `previous` and `hiding` describe a state that no longer exists. Applying
    // either answer would write against stale assumptions.
    it('reports superseded when the form reloads mid-dialog', async () => {
      const { policy, form, applied, generation } = setup({
        form: { route: 'aanbesteding', deadline: '2026-09-15' },
        visibleFor: routeRule,
        policyFor: (p) => (p === 'deadline' ? 'confirm' : 'no'),
        approve: true,
      })

      const pending = policy.propose('route', 'onderhands', 'aanbesteding')
      generation.value++ // entity reloaded while the dialog was open
      const outcome = await pending

      expect(outcome).toEqual({ status: 'superseded' })
      expect(applied).toEqual([])
      expect(form.route).toBe('aanbesteding')
    })

    it('names every confirm-policy field in one dialog, not one each', async () => {
      const { policy, asked } = setup({
        form: { route: 'aanbesteding', a: 'x', b: 'y' },
        visibleFor: (f) => (f.route === 'aanbesteding' ? ['route', 'a', 'b'] : ['route']),
        policyFor: (p) => (p === 'route' ? 'no' : 'confirm'),
        approve: true,
      })

      await policy.propose('route', 'onderhands', 'aanbesteding')

      expect(asked).toHaveLength(1)
      expect(asked[0].sort()).toEqual(['a', 'b'])
    })
  })

  // hidesFrom is the question asked before deciding; asking must be free.
  it('hidesFrom does not mutate or apply anything', async () => {
    const apply = vi.fn()
    const form = { route: 'aanbesteding', deadline: '2026-09-15' }
    const policy = useChangePolicy({
      bindings: () => ({ form, entity: {}, current_user: {} }),
      activeNow: () => new Set(routeRule(form)),
      activeFor: (b) => new Set(routeRule((b.form ?? {}) as Record<string, unknown>)),
      managed: () => new Set(['route', 'deadline']),
      valueOf: (p) => (form as Record<string, unknown>)[p],
      retain: vi.fn(),
      apply,
      onHidden: vi.fn(),
      enabled: () => true,
      policyFor: () => 'no',
      askToClear: async () => true,
      isEmpty: () => false,
      generation: () => 0,
    })

    const hiding = policy.hidesFrom({ property: 'route', value: 'onderhands', previous: 'aanbesteding' })

    expect(hiding).toEqual(['deadline'])
    expect(apply).not.toHaveBeenCalled()
    expect(form).toEqual({ route: 'aanbesteding', deadline: '2026-09-15' })
  })
})
