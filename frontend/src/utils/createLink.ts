/**
 * The pre-link contract between a "create related entity" button and the create
 * form it opens.
 *
 * Both surfaces that offer such a button — the entity-detail view's section
 * affordance and the edit form's side panel — go through here, because they
 * previously did not and drifted: `SidePanel.vue` pushed `_relation` /
 * `_linkAs` / `_peerId` while `DynamicForm.vue` read `link_relation` /
 * `link_peer` / `link_as`. Nothing read the underscore spelling, so that button
 * opened a form with no relation context at all and the user had to link by
 * hand anyway — the exact chore it existed to remove (TKT-R4BMJM).
 *
 * Keeping the names in one module means a rename breaks the build rather than
 * silently un-linking a button.
 */

/** The role the NEWLY created entity takes in the relation. */
export type LinkAs = 'from' | 'to'

export interface CreateLinkParams {
  /** Relation type the new entity is linked by. */
  relation: string
  /** The existing entity on the other end. */
  peer: string
  /** Role of the new entity: 'to' for an outgoing section, 'from' for incoming. */
  linkAs: LinkAs
  /** Path to return to after create; omitted when the caller wants the default. */
  returnTo?: string
  /** Entity template variant to preselect, or undefined for the form's default. */
  template?: string
  /** World to create in, carried so a faced type lands on the right face. */
  world?: string
}

/**
 * Query-param names DynamicForm reads. Exported so a test can assert the
 * emitted URL uses the spelling the form consumes, rather than re-stating the
 * literals and passing against a mismatch.
 */
export const LINK_QUERY_KEYS = {
  relation: 'link_relation',
  peer: 'link_peer',
  linkAs: 'link_as',
  returnTo: 'return_to',
  template: 'template',
  world: 'world',
} as const

/**
 * Builds the query for a page-flow create, pre-linking the new entity.
 *
 * These values are a CONVENIENCE, not a capability: they land in an editable
 * URL, and the server re-authorizes both the create and the edge. A user who
 * rewrites them gets whatever their own permissions allow, which is what they
 * would get by using the create form directly.
 */
export function buildCreateLinkQuery(params: CreateLinkParams): Record<string, string> {
  const query: Record<string, string> = {
    [LINK_QUERY_KEYS.relation]: params.relation,
    [LINK_QUERY_KEYS.peer]: params.peer,
    [LINK_QUERY_KEYS.linkAs]: params.linkAs,
  }
  if (params.returnTo) query[LINK_QUERY_KEYS.returnTo] = params.returnTo
  if (params.template) query[LINK_QUERY_KEYS.template] = params.template
  if (params.world) query[LINK_QUERY_KEYS.world] = params.world
  return query
}
