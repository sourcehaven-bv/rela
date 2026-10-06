// Helper for computing the canonical SPA path for an entity link.
//
// Priority chain:
//   1. opts.cellLink (server-resolved per-column link, e.g. table cells)
//   2. /entity/<entity.type>/<entity.id> floor (always non-empty when type is)
//
// Returns an empty string when entity.type is empty — templates must guard
// with v-if="href" and skip rendering an anchor in that case (otherwise we
// would emit /entity//<id>, which 404s).

export interface EntityRef {
  id: string
  type: string
}

export interface EntityDetailHrefOpts {
  cellLink?: string
}

export function entityDetailHref(
  entity: EntityRef,
  opts: EntityDetailHrefOpts = {},
): string {
  if (opts.cellLink) return opts.cellLink
  if (!entity.type || !entity.id) return ''
  return `/entity/${entity.type}/${entity.id}`
}

// The route to an entity's edit form, in the world the caller is showing.
//
// `address` names the row (`POL-1@draft`), but the form also loads that row's
// relations, and the server resolves those per world: a peer with no face in
// the world is left out. A form opened without the page's world therefore
// loads in the default world, and a relation the page showed is missing from
// the form. Every edit entry point builds its route here so none can drop the
// world. `world` is `useWorld().worldParam`: undefined for the default world.
export function editFormRoute(
  formId: string,
  address: string,
  world?: string,
  query: Record<string, string> = {},
): EditFormRoute {
  return {
    name: 'form-edit',
    params: { id: formId, entityId: address },
    query: world ? { ...query, world } : { ...query },
  }
}

export interface EditFormRoute {
  name: 'form-edit'
  params: { id: string; entityId: string }
  query: Record<string, string>
}
