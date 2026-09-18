/**
 * The toolbar glyphs, as data.
 *
 * Drawn on a 24x24 grid with `currentColor`, so one glyph works in both themes
 * and inherits the button's hover and active colours. Geometry is hand-written
 * rather than copied from an icon set, so there is no licence to carry.
 *
 * Data rather than markup because there are two renderers. The SPA draws them
 * from a Vue component (`BlockIcon.vue`); the sandboxed app editor is a plain
 * IIFE with no framework and builds the same SVG through the DOM API. Both read
 * this table, so a glyph added for one appears in the other and neither can
 * drift into its own private set.
 */

/** One SVG child element: its tag and its attributes, or a text label. */
export interface IconPart {
  tag: 'path' | 'circle' | 'rect' | 'text'
  attrs: Record<string, string | number>
  /** Text content, for the `text` tag only. */
  text?: string
}

/** The shared attributes of the `<svg>` wrapper both renderers produce. */
export const ICON_SVG_ATTRS: Record<string, string> = {
  xmlns: 'http://www.w3.org/2000/svg',
  viewBox: '0 0 24 24',
  width: '16',
  height: '16',
  fill: 'none',
  stroke: 'currentColor',
  'stroke-width': '2',
  'stroke-linecap': 'round',
  'stroke-linejoin': 'round',
  'aria-hidden': 'true',
}

/** A filled glyph element, which opts out of the wrapper's stroke. */
const filled = { fill: 'currentColor', stroke: 'none' } as const

/** The numeral drawn beside the H in a heading glyph. */
function headingGlyph(level: string): IconPart[] {
  return [
    { tag: 'path', attrs: { d: 'M5 6v12M13 6v12M5 12h8' } },
    {
      tag: 'text',
      attrs: {
        x: 16.5,
        y: 18,
        'font-size': 10,
        'font-family': 'inherit',
        'font-weight': 700,
        ...filled,
      },
      text: level,
    },
  ]
}

/** A numbered-list numeral. */
function numeral(y: number, text: string): IconPart {
  return {
    tag: 'text',
    attrs: { x: 2, y, 'font-size': 7, 'font-family': 'inherit', ...filled },
    text,
  }
}

/**
 * Glyph geometry by command id.
 *
 * Keys are `EditorCommand.id` values, plus `entityRef` for the reference
 * button, which is not a formatting command.
 */
export const ICON_PARTS: Record<string, IconPart[]> = {
  // Bold: a B built from two stacked bowls.
  strong: [
    { tag: 'path', attrs: { d: 'M7 5h6a3.5 3.5 0 0 1 0 7H7z' } },
    { tag: 'path', attrs: { d: 'M7 12h7a3.5 3.5 0 0 1 0 7H7z' } },
  ],
  // Italic: slanted stroke with serifs.
  emphasis: [{ tag: 'path', attrs: { d: 'M15 5h-5M14 19H9M14 5l-4 14' } }],
  // Strikethrough: an S-ish stroke crossed through the middle.
  strikethrough: [
    { tag: 'path', attrs: { d: 'M4 12h16' } },
    { tag: 'path', attrs: { d: 'M16 7a4 4 0 0 0-4-2c-2.2 0-4 1.2-4 3 0 1.3.8 2.2 2 2.8' } },
    { tag: 'path', attrs: { d: 'M8 17a4 4 0 0 0 4 2c2.2 0 4-1.2 4-3 0-.6-.2-1.2-.6-1.6' } },
  ],
  // Inline code: angle brackets.
  inlineCode: [{ tag: 'path', attrs: { d: 'm9 8-4 4 4 4M15 8l4 4-4 4' } }],

  h1: headingGlyph('1'),
  h2: headingGlyph('2'),
  h3: headingGlyph('3'),

  // Bullet list: dots plus rules.
  bulletList: [
    { tag: 'path', attrs: { d: 'M9 6h11M9 12h11M9 18h11' } },
    { tag: 'circle', attrs: { cx: 4.5, cy: 6, r: 1.4, ...filled } },
    { tag: 'circle', attrs: { cx: 4.5, cy: 12, r: 1.4, ...filled } },
    { tag: 'circle', attrs: { cx: 4.5, cy: 18, r: 1.4, ...filled } },
  ],
  // Numbered list: numerals plus rules.
  orderedList: [
    { tag: 'path', attrs: { d: 'M10 6h10M10 12h10M10 18h10' } },
    numeral(8, '1'),
    numeral(14, '2'),
    numeral(20, '3'),
  ],
  // Task list: checkboxes plus rules, the first one ticked.
  taskList: [
    { tag: 'path', attrs: { d: 'M10 6h10M10 12h10M10 18h10' } },
    { tag: 'rect', attrs: { x: 2, y: 3.6, width: 5, height: 5, rx: 1 } },
    { tag: 'path', attrs: { d: 'M3 6.1l1.3 1.3L6.2 5' } },
    { tag: 'rect', attrs: { x: 2, y: 9.6, width: 5, height: 5, rx: 1 } },
    { tag: 'rect', attrs: { x: 2, y: 15.6, width: 5, height: 5, rx: 1 } },
  ],

  // Quote: a bar with indented rules.
  blockquote: [
    { tag: 'path', attrs: { d: 'M5 5v14', 'stroke-width': 2.5 } },
    { tag: 'path', attrs: { d: 'M10 8h9M10 12h9M10 16h6' } },
  ],
  // Code block: brackets inside a frame.
  codeBlock: [
    { tag: 'rect', attrs: { x: 3, y: 4, width: 18, height: 16, rx: 2 } },
    { tag: 'path', attrs: { d: 'm10 10-2 2 2 2M14 10l2 2-2 2' } },
  ],
  // Table: a grid with a header row.
  table: [
    { tag: 'rect', attrs: { x: 3, y: 4, width: 18, height: 16, rx: 2 } },
    { tag: 'path', attrs: { d: 'M3 9h18M9 9v11M15 9v11' } },
  ],

  // Link: the two half-links of a chain, meeting at a bar.
  link: [
    { tag: 'path', attrs: { d: 'M10 13a5 5 0 0 0 7 0l2-2a5 5 0 0 0-7-7l-1 1' } },
    { tag: 'path', attrs: { d: 'M14 11a5 5 0 0 0-7 0l-2 2a5 5 0 0 0 7 7l1-1' } },
  ],
  // Unlink: the same chain, broken, with the gap made explicit.
  unlink: [
    { tag: 'path', attrs: { d: 'M16 13l1-1a5 5 0 0 0-7-7l-1 1' } },
    { tag: 'path', attrs: { d: 'M8 11l-1 1a5 5 0 0 0 7 7l1-1' } },
    { tag: 'path', attrs: { d: 'm4 4 16 16' } },
  ],
  // Divider: a full-width rule, with the text it separates implied.
  hr: [
    { tag: 'path', attrs: { d: 'M3 12h18', 'stroke-width': 2.5 } },
    { tag: 'path', attrs: { d: 'M6 6h12M6 18h12', opacity: 0.45 } },
  ],
  // Undo / redo: an arrow curving back onto the line it came from.
  undo: [
    { tag: 'path', attrs: { d: 'M9 14 4 9l5-5' } },
    { tag: 'path', attrs: { d: 'M4 9h10a6 6 0 0 1 0 12h-3' } },
  ],
  redo: [
    { tag: 'path', attrs: { d: 'm15 14 5-5-5-5' } },
    { tag: 'path', attrs: { d: 'M20 9H10a6 6 0 0 0 0 12h3' } },
  ],

  // Table row/column operations. Each shows the grid with the affected band
  // highlighted and a +/x marking what happens to it.
  addRowBefore: [
    { tag: 'rect', attrs: { x: 3, y: 10, width: 18, height: 11, rx: 1.5 } },
    { tag: 'path', attrs: { d: 'M3 15.5h18' } },
    { tag: 'path', attrs: { d: 'M12 3v5M9.5 5.5h5' } },
  ],
  addRowAfter: [
    { tag: 'rect', attrs: { x: 3, y: 3, width: 18, height: 11, rx: 1.5 } },
    { tag: 'path', attrs: { d: 'M3 8.5h18' } },
    { tag: 'path', attrs: { d: 'M12 16v5M9.5 18.5h5' } },
  ],
  addColBefore: [
    { tag: 'rect', attrs: { x: 10, y: 3, width: 11, height: 18, rx: 1.5 } },
    { tag: 'path', attrs: { d: 'M15.5 3v18' } },
    { tag: 'path', attrs: { d: 'M3 12h5M5.5 9.5v5' } },
  ],
  addColAfter: [
    { tag: 'rect', attrs: { x: 3, y: 3, width: 11, height: 18, rx: 1.5 } },
    { tag: 'path', attrs: { d: 'M8.5 3v18' } },
    { tag: 'path', attrs: { d: 'M16 12h5M18.5 9.5v5' } },
  ],
  deleteRow: [
    { tag: 'rect', attrs: { x: 3, y: 4, width: 18, height: 16, rx: 1.5 } },
    { tag: 'path', attrs: { d: 'M3 9.5h18M3 14.5h18' } },
    { tag: 'path', attrs: { d: 'M8 12h8', 'stroke-width': 2.5 } },
  ],
  deleteColumn: [
    { tag: 'rect', attrs: { x: 4, y: 3, width: 16, height: 18, rx: 1.5 } },
    { tag: 'path', attrs: { d: 'M9.5 3v18M14.5 3v18' } },
    { tag: 'path', attrs: { d: 'M12 8v8', 'stroke-width': 2.5 } },
  ],
  deleteTable: [
    { tag: 'rect', attrs: { x: 3, y: 4, width: 18, height: 16, rx: 1.5 } },
    { tag: 'path', attrs: { d: 'M3 9h18M9 9v11' } },
    { tag: 'path', attrs: { d: 'm14 13 5 5M19 13l-5 5' } },
  ],

  // Entity reference: connected nodes, the graph-edge idea.
  entityRef: [
    { tag: 'circle', attrs: { cx: 5, cy: 6, r: 2 } },
    { tag: 'circle', attrs: { cx: 19, cy: 9, r: 2 } },
    { tag: 'circle', attrs: { cx: 12, cy: 19, r: 2 } },
    { tag: 'path', attrs: { d: 'm6.8 7 10.4 1.4M17.6 10.7 13.2 17.2M10.6 17.4 6 7.9' } },
  ],
}

const SVG_NS = 'http://www.w3.org/2000/svg'

/**
 * Builds a glyph as a detached `<svg>` element.
 *
 * For the plain-DOM renderer. Returns an empty `<svg>` for an unknown name
 * rather than throwing, matching the Vue component, whose `v-else-if` chain
 * simply falls through: a missing glyph should leave a blank button, not break
 * the toolbar around it.
 */
export function createIconSvg(name: string): SVGSVGElement {
  const svg = document.createElementNS(SVG_NS, 'svg')
  for (const [k, v] of Object.entries(ICON_SVG_ATTRS)) svg.setAttribute(k, v)
  for (const part of ICON_PARTS[name] ?? []) {
    const el = document.createElementNS(SVG_NS, part.tag)
    for (const [k, v] of Object.entries(part.attrs)) el.setAttribute(k, String(v))
    if (part.text !== undefined) el.textContent = part.text
    svg.appendChild(el)
  }
  return svg
}
