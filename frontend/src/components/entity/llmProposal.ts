import type { ActionProposal } from '@/api/actions'

/** Writes the proposal dialog makes, after the user's choice. */
export interface ProposalPlan {
  /** Properties to PATCH, by name. */
  properties: Record<string, unknown>
  /** Properties left out because the entity changed since the proposal. */
  stale: string[]
  /** Relations to create. */
  relations: { relation: string; to: string; title: string }[]
}

/** Key of a proposed relation in the dialog's selection. */
export function relationKey(r: { relation: string; to: string }): string {
  return `${r.relation}\u0000${r.to}`
}

function same(a: unknown, b: unknown): boolean {
  return JSON.stringify(a ?? null) === JSON.stringify(b ?? null)
}

/**
 * Plans the writes for the selected changes. A property whose value on the
 * freshly read entity differs from the value the proposal was made against is
 * left out and reported as stale, so a proposal never overwrites an edit made
 * while the model was answering.
 */
export function planApply(
  proposal: ActionProposal,
  selectedSet: ReadonlySet<string>,
  selectedRelate: ReadonlySet<string>,
  freshProperties: Record<string, unknown>
): ProposalPlan {
  const plan: ProposalPlan = { properties: {}, stale: [], relations: [] }
  for (const c of proposal.set) {
    if (!selectedSet.has(c.property)) continue
    if (!same(freshProperties[c.property], c.current)) {
      plan.stale.push(c.property)
      continue
    }
    plan.properties[c.property] = c.proposed
  }
  for (const r of proposal.relate) {
    if (selectedRelate.has(relationKey(r))) {
      plan.relations.push({ relation: r.relation, to: r.to, title: r.to_title })
    }
  }
  return plan
}

/** Renders a property value for the dialog. */
export function formatProposalValue(v: unknown): string {
  if (v === null || v === undefined || v === '') return '—'
  if (Array.isArray(v)) return v.map((x) => formatProposalValue(x)).join(', ')
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}
