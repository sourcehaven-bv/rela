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

export interface OwnedEntityRef extends EntityRef {
  _owner?: EntityRef
}

// ownedEntityHref links an entity that may be owned (TKT-QO14GB): to its
// owner's page, anchored at the entity, when it has an owner, and to its own
// page otherwise.
export function ownedEntityHref(entity: OwnedEntityRef): string {
  if (entity._owner) {
    const base = entityDetailHref(entity._owner)
    return base && `${base}#${encodeURIComponent(entity.id)}`
  }
  return entityDetailHref(entity)
}

export interface EntityDetailHrefOpts {
  cellLink?: string
}

export function entityDetailHref(entity: EntityRef, opts: EntityDetailHrefOpts = {}): string {
  if (opts.cellLink) return opts.cellLink
  if (!entity.type || !entity.id) return ''
  return `/entity/${entity.type}/${entity.id}`
}
