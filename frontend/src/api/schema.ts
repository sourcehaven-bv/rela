import { api } from './client'
import type { Schema, Config, SidebarData } from '@/types'

export async function getSchema(): Promise<Schema> {
  return api.get<Schema>('/_schema')
}

export async function getConfig(): Promise<Config> {
  return api.get<Config>('/_config')
}

/**
 * The sidebar for one space. Without a space, or with one the principal may
 * not enter, the server picks the first space they may; the response names it.
 */
export async function getSidebar(space?: string): Promise<SidebarData> {
  return api.get<SidebarData>('/_sidebar', space ? { space } : undefined)
}
