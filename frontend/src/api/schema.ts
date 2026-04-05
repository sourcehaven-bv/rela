import { api } from './client'
import type { Schema, Config, SidebarData } from '@/types'

export async function getSchema(): Promise<Schema> {
  return api.get<Schema>('/_schema')
}

export async function getConfig(): Promise<Config> {
  return api.get<Config>('/_config')
}

export async function getSidebar(): Promise<SidebarData> {
  return api.get<SidebarData>('/_sidebar')
}
