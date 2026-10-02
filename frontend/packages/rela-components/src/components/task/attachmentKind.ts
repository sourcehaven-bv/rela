import type { AttachmentKind } from '../../types'

const byExtension: Record<string, AttachmentKind> = {
  pdf: 'pdf',
  doc: 'doc',
  docx: 'doc',
  odt: 'doc',
  rtf: 'doc',
  txt: 'doc',
  md: 'doc',
  xls: 'sheet',
  xlsx: 'sheet',
  ods: 'sheet',
  csv: 'sheet',
  png: 'image',
  jpg: 'image',
  jpeg: 'image',
  gif: 'image',
  webp: 'image',
  svg: 'image',
}

/**
 * The kind of a file, which sets its card's colour and icon. The content type
 * wins when it names an image or a PDF; otherwise the extension decides,
 * because office formats hide behind long vendor types.
 */
export function attachmentKind(name: string, contentType?: string): AttachmentKind {
  if (contentType?.startsWith('image/')) return 'image'
  if (contentType === 'application/pdf') return 'pdf'
  const ext = name.includes('.') ? name.split('.').pop()!.toLowerCase() : ''
  return byExtension[ext] ?? 'other'
}
