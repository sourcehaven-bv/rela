/**
 * How the Configure space names things.
 *
 * Labels are authored, never derived from identifiers (DEC-6C1NAA): a screen
 * shows the `label:` the configuration gives, and the identifier unchanged
 * when there is none.
 */

/**
 * A machine name from a label, for something new: `Due date` → `due_date`.
 * `separator` follows the file's own habit for the kind of thing named.
 */
export function machineName(label: string, separator: '_' | '-' = '_'): string {
  return label
    .trim()
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z0-9]+/g, separator)
    .replace(new RegExp(`^\\${separator}+|\\${separator}+$`, 'g'), '')
}

/** What a property's type is called on screen, for the built-in types. */
export const BUILTIN_TYPES: { value: string; label: string }[] = [
  { value: 'string', label: 'Text' },
  { value: 'integer', label: 'Number' },
  { value: 'boolean', label: 'Yes / no' },
  { value: 'date', label: 'Date' },
  { value: 'datetime', label: 'Date and time' },
  { value: 'file', label: 'File' },
  { value: 'rrule', label: 'Repeats' },
]

export function builtinTypeLabel(type: string): string | undefined {
  return BUILTIN_TYPES.find((t) => t.value === type)?.label
}

/**
 * The colours the picker offers, as data-entry styles name them. A kit shade
 * (`red/700`) or an operator colour is written in data-entry.yaml; the picker
 * keeps such a value and previews it by its hue.
 */
export const OPTION_COLORS = [
  'gray',
  'blue',
  'teal',
  'green',
  'yellow',
  'orange',
  'red',
  'purple',
] as const

export type TagColor = 'grey' | 'blue' | 'green' | 'amber' | 'yellow' | 'teal' | 'red' | 'purple'

/**
 * A style colour as the library's tag draws it. A shade draws as its hue's
 * tag; an operator colour and an unknown name draw grey.
 */
export function tagColor(style: string | undefined): TagColor {
  const hue = style?.split('/')[0]
  switch (hue) {
    case 'blue':
    case 'green':
    case 'yellow':
    case 'teal':
    case 'red':
    case 'purple':
      return hue
    case 'orange':
      return 'amber'
    default:
      return 'grey'
  }
}

export function colorLabel(style: string | undefined): string {
  return style || 'No colour'
}

/** `1617` → `1,617 records`. */
export function countOf(n: number, one: string, many: string): string {
  return `${n.toLocaleString('en')} ${n === 1 ? one : many}`
}

/** Colour presets for an entity type: a light fill and a darker border. */
export const ENTITY_COLORS: { name: string; color: string; border: string }[] = [
  { name: 'blue', color: '#E3F2FD', border: '#1976D2' },
  { name: 'purple', color: '#F3E5F5', border: '#9C27B0' },
  { name: 'green', color: '#E8F5E9', border: '#388E3C' },
  { name: 'orange', color: '#FFF3E0', border: '#F57C00' },
  { name: 'red', color: '#FFEBEE', border: '#D32F2F' },
  { name: 'gray', color: '#F5F5F5', border: '#616161' },
]

/** The preset an entity type's colours match, if any. */
export function entityColorName(color: string, border: string): string {
  const found = ENTITY_COLORS.find(
    (c) =>
      c.color.toLowerCase() === color.toLowerCase() &&
      (!border || c.border.toLowerCase() === border.toLowerCase())
  )
  if (found) return found.name
  return color ? 'custom' : ''
}
