// Canonical helpers for displaying an entity's name in the SPA.
//
// The backend computes a metamodel-aware display name into `_title`
// (api_v1.go entityToV1 -> metamodel.DisplayTitle), honoring each entity
// type's `display_property`. `_title` is total: it is the resolved
// display value, or the entity ID as a floor — never empty for an
// API-sourced entity.
//
// Components MUST render `_title` rather than reaching into
// `properties.title`. The literal property name `title` is only correct
// for types whose `display_property` happens to be `title`; for any other
// (e.g. `naam`), `properties.title` is empty and the UI falls back to the
// bare ID. That divergence was BUG-1P88YM. An ESLint guard
// (no-restricted-syntax) bans `.properties.title` in display code to keep
// every render site funneled through here.

export interface EntityDisplayLike {
  id: string
  _title?: string
}

// entityDisplayTitle returns the entity's display name: `_title` when the
// backend supplied one, else the ID. Use this everywhere an entity name
// is shown (pickers, cards, lists, search rows, detail headings).
export function entityDisplayTitle(entity: EntityDisplayLike): string {
  return entity._title && entity._title !== '' ? entity._title : entity.id
}

// entityDisplayTitleWithId returns "Title (ID)" when the title differs
// from the ID, else just the ID. Used by widgets that show both the
// human name and the raw ID (e.g. RelationPicker chips/options).
export function entityDisplayTitleWithId(entity: EntityDisplayLike): string {
  const title = entityDisplayTitle(entity)
  return title !== entity.id ? `${title} (${entity.id})` : entity.id
}
