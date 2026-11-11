/** The list key of a channel; a `one` type has no name. */
export function channelKey(type: string, name?: string): string {
  return `${type}/${name ?? ''}`
}
