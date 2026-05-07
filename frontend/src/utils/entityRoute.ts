// Helper for computing the canonical SPA path for an entity link.
//
// Priority chain:
//   1. opts.cellLink (server-resolved per-column link, e.g. table cells)
//   2. /view/<detailView>/<entity.id> when a detail_view is configured
//      for the entity's type via entity_views.<type>.detail_view
//   3. /entity/<entity.type>/<entity.id> floor (always non-empty when type is)
//
// Returns an empty string when entity.type is empty — templates must guard
// with v-if="href" and skip rendering an anchor in that case (otherwise we
// would emit /entity//<id>, which 404s).
//
// The helper takes a getDetailView callback rather than the schema store so
// it stays testable without Pinia. Consumers in components import the store
// and pass `(type) => schemaStore.getEntityDetailView(type)`.
//
// TODO: a future iteration could move this resolution server-side, having the
// API emit `entity.detailHref` per item so the SPA renders <a :href> with no
// client-side lookup. Tracked separately.

export interface EntityRef {
  id: string
  type: string
}

export interface EntityDetailHrefOpts {
  cellLink?: string
}

export type GetDetailView = (type: string) => string | undefined

export function entityDetailHref(
  entity: EntityRef,
  getDetailView: GetDetailView,
  opts: EntityDetailHrefOpts = {}
): string {
  if (opts.cellLink) return opts.cellLink
  if (!entity.type || !entity.id) return ''
  const detailView = getDetailView(entity.type)
  if (detailView) return `/view/${detailView}/${entity.id}`
  return `/entity/${entity.type}/${entity.id}`
}
