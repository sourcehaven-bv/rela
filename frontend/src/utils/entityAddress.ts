/**
 * The shape of an entity address, `id` or `id@face`. It admits no `/`,
 * `?`, `#` or `%`, so an address that passes cannot move a request to
 * another API path.
 */
export const ENTITY_ADDRESS = /^[A-Za-z0-9][A-Za-z0-9_.-]*(?:@[A-Za-z0-9][A-Za-z0-9_.-]*)?$/

/**
 * Returns address for use as one path segment, or throws when it does not
 * have the shape of an entity address. A path builder calls this rather
 * than interpolating an address that may come from a route parameter.
 */
export function addressSegment(address: string): string {
  if (!ENTITY_ADDRESS.test(address)) {
    throw new Error(`not an entity address: ${JSON.stringify(address)}`)
  }
  return address
}
