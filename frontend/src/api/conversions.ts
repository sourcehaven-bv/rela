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
  const preview = await api.post<ConversionPreview>(
    `/_conversions/${encodeURIComponent(name)}/preflight`,
    request
  )
  return normalizePreview(preview)
}

// normalizePreview turns absent or null list fields into empty arrays. Older
// servers sent null for an empty list, and the dialog reads .length on them.
export function normalizePreview(preview: ConversionPreview): ConversionPreview {
  return {
    ...preview,
    required: preview.required ?? [],
    required_faces: preview.required_faces ?? [],
    relations_to_drop: preview.relations_to_drop ?? [],
    invalid_relations: preview.invalid_relations ?? [],
    target_faces: preview.target_faces ?? [],
    invalid_faces: preview.invalid_faces ?? [],
    moves: preview.moves ?? [],
  }
}

export async function convertEntity(
  name: string,
  request: ConvertRequest
): Promise<ConversionResult> {
  return api.post<ConversionResult>(`/_conversions/${encodeURIComponent(name)}`, request)
}

export type { ConversionOffer }
