import { api } from './client'
import type { ConversionOffer } from '@/types'

export interface ConversionResult {
  definition: string
  old_id: string
  entity_id: string
  face: string
  relations_dropped: number
}

export interface ConvertRequest {
  source_id: string
  source_face?: string
  target_id?: string
  properties?: Record<string, unknown>
  content?: string
  drop_relations?: string[]
  faces?: Record<string, string>
}

export interface ConversionPreview {
  target_type: string
  required: string[]
  required_faces?: string[]
  relations_to_drop: string[]
  invalid_relations: string[]
  target_faces: string[]
  invalid_faces?: string[]
  // Where each source face's comments go. Readers of the target face can
  // read them after the conversion.
  moves: ConversionMove[]
}

export interface ConversionMove {
  source_face: string
  target_face: string
}

export async function previewConversion(
  name: string,
  request: ConvertRequest
): Promise<ConversionPreview> {
  return api.post<ConversionPreview>(`/_conversions/${encodeURIComponent(name)}/preflight`, request)
}

export async function convertEntity(
  name: string,
  request: ConvertRequest
): Promise<ConversionResult> {
  return api.post<ConversionResult>(`/_conversions/${encodeURIComponent(name)}`, request)
}

export type { ConversionOffer }
