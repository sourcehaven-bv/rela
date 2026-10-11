/**
 * Where each Configure screen lives. Every link in the space, and every link
 * from a change or a problem back to its screen, is built here.
 *
 * Configure routes are never prefixed with a space (`/s/<space>`): the
 * Configure space sits beside the spaces it configures, not inside one.
 */
const seg = encodeURIComponent

export const CONFIGURE_PREFIX = '/configure'

export const configureRoute = {
  home: () => CONFIGURE_PREFIX,
  entityTypes: () => `${CONFIGURE_PREFIX}/entity-types`,
  entityType: (name: string) => `${CONFIGURE_PREFIX}/entity-types/${seg(name)}`,
  choiceLists: () => `${CONFIGURE_PREFIX}/choice-lists`,
  choiceList: (name: string) => `${CONFIGURE_PREFIX}/choice-lists/${seg(name)}`,
  relations: () => `${CONFIGURE_PREFIX}/relations`,
  relation: (name: string) => `${CONFIGURE_PREFIX}/relations/${seg(name)}`,
  rules: () => `${CONFIGURE_PREFIX}/rules`,
  rule: (index: number) => `${CONFIGURE_PREFIX}/rules/${index}`,
  automations: () => `${CONFIGURE_PREFIX}/automations`,
  automation: (index: number) => `${CONFIGURE_PREFIX}/automations/${index}`,
  navigation: (space?: string) =>
    space ? `${CONFIGURE_PREFIX}/navigation?space=${seg(space)}` : `${CONFIGURE_PREFIX}/navigation`,
  forms: () => `${CONFIGURE_PREFIX}/forms`,
  form: (name: string) => `${CONFIGURE_PREFIX}/forms/${seg(name)}`,
  lists: () => `${CONFIGURE_PREFIX}/lists`,
  list: (name: string) => `${CONFIGURE_PREFIX}/lists/${seg(name)}`,
  boards: () => `${CONFIGURE_PREFIX}/boards`,
  board: (name: string) => `${CONFIGURE_PREFIX}/boards/${seg(name)}`,
  dashboard: () => `${CONFIGURE_PREFIX}/dashboard`,
}

export function isConfigurePath(path: string): boolean {
  return path === CONFIGURE_PREFIX || path.startsWith(`${CONFIGURE_PREFIX}/`)
}
