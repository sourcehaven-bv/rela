/**
 * The ADDRESS of the row a response describes: `POL-1` for the bare face,
 * `POL-1@published` for a non-bare one.
 *
 * ## The face is part of the address, exactly as the id is
 *
 *     view[entity@face]  --Edit-->  form[entity@face]  --Save-->  PATCH entity@face
 *
 * Every read carries the coordinate of the row on screen in `_self`; every
 * write the SPA makes must go to that same coordinate, or it edits a state
 * the page is not showing. Under a world a bare id resolves to whichever
 * face the world picks, so the bare id is NOT a stable address for the row
 * on screen — `_self` is. The server accepts the same spelling on GET,
 * PATCH and DELETE and on the `_views` route (TKT-SLFURL).
 *
 * The SPA never derives this itself. It does not know which face a world
 * resolved, whether the type has a `bare_face`, or how the server spells a
 * coordinate; it reads the answer off the response. A response with no
 * `_self` (an older server, a synthetic entity) falls back to the id, which
 * is the pre-worlds behaviour and correct for every unfaced type.
 */
export function entityRef(e: { id: string; _self?: string }): string {
  const self = e._self
  if (!self) return e.id
  const last = self.slice(self.lastIndexOf('/') + 1)
  if (!last) return e.id
  try {
    return decodeURIComponent(last)
  } catch {
    return e.id
  }
}

/** The bare entity id of an address: `POL-1@published` → `POL-1`. */
export function refBareId(ref: string): string {
  const at = ref.indexOf('@')
  return at < 0 ? ref : ref.slice(0, at)
}

/** The face an address names, or '' for the bare id. */
export function refFace(ref: string): string {
  const at = ref.indexOf('@')
  return at < 0 ? '' : ref.slice(at + 1)
}
