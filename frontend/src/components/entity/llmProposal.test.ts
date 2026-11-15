import { describe, expect, it } from 'vitest'
import { formatProposalValue, planApply, relationKey } from './llmProposal'
import type { ActionProposal } from '@/api/actions'

const proposal: ActionProposal = {
  entity: 'TKT-1',
  set: [
    { property: 'priority', current: 'low', proposed: 'high' },
    { property: 'summary', current: null, proposed: 'Login fails' },
  ],
  relate: [
    { relation: 'affects', to: 'CON-1', to_type: 'concept', to_title: 'Sessions' },
    { relation: 'affects', to: 'CON-2', to_type: 'concept', to_title: 'Tokens' },
  ],
}

describe('planApply', () => {
  it('plans only the selected changes', () => {
    const plan = planApply(
      proposal,
      new Set(['priority']),
      new Set([relationKey(proposal.relate[1])]),
      { priority: 'low' }
    )
    expect(plan.properties).toEqual({ priority: 'high' })
    expect(plan.stale).toEqual([])
    expect(plan.relations).toEqual([{ relation: 'affects', to: 'CON-2', title: 'Tokens' }])
  })

  it('skips a property that changed since the proposal', () => {
    const plan = planApply(proposal, new Set(['priority', 'summary']), new Set(), {
      priority: 'medium',
    })
    expect(plan.stale).toEqual(['priority'])
    // An unset value matches a null current.
    expect(plan.properties).toEqual({ summary: 'Login fails' })
  })
})

describe('formatProposalValue', () => {
  it('renders empty, lists and scalars', () => {
    expect(formatProposalValue(null)).toBe('—')
    expect(formatProposalValue(['a', 'b'])).toBe('a, b')
    expect(formatProposalValue(3)).toBe('3')
  })
})
