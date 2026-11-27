/** The id prefix of a pile's sidebar row; the rest is the pile id. */
export const PILE_NAV_PREFIX = 'pile:'

/** The sidebar row id of a pile, shared by the sidebar and the flyout. */
export function pileNavId(id: string): string {
  return `${PILE_NAV_PREFIX}${id}`
}
