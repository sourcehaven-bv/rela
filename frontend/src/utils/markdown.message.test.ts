// @vitest-environment jsdom
//
// jsdom for the same reason as markdown.test.ts: DOMPurify needs
// browser-accurate serialization (BUG-SQSV6V).
import { describe, it, expect } from 'vitest'
import { renderMessageMarkdown } from './markdown'

// Channel messages come from any member, so their renderer keeps a strict
// allowlist (TKT-91BK6F R2, R3): nothing in the output may load a remote
// resource, run script, restyle the app or point an in-app looking link
// somewhere the server did not resolve.

function dom(html: string): HTMLTemplateElement {
  const t = document.createElement('template')
  t.innerHTML = html
  return t
}

describe('renderMessageMarkdown', () => {
  it('renders the allowed markdown', () => {
    const html = renderMessageMarkdown(
      '# Title\n\n**bold** _em_ ~~del~~ `code`\n\n> quote\n\n- a\n- b\n\n```\npre\n```\n\n---'
    )
    const t = dom(html)
    for (const tag of [
      'h1',
      'strong',
      'em',
      'del',
      'code',
      'blockquote',
      'ul',
      'li',
      'pre',
      'hr',
    ]) {
      expect(t.content.querySelector(tag), tag).not.toBeNull()
    }
  })

  it('keeps a table, wrapped for scrolling', () => {
    const t = dom(renderMessageMarkdown('| a | b |\n|---|---|\n| 1 | 2 |'))
    expect(t.content.querySelector('.md-table-scroll > table td')?.textContent).toBe('1')
  })

  it('treats a newline as a line break', () => {
    expect(renderMessageMarkdown('one\ntwo')).toContain('<br>')
  })

  it.each([
    ['an image', '![x](https://evil.example/p.png)', 'img'],
    ['raw img', '<img src="https://evil.example/p.png">', 'img'],
    ['a script', '<script>alert(1)</script>', 'script'],
    ['an iframe', '<iframe src="https://evil.example"></iframe>', 'iframe'],
    ['svg', '<svg><circle r="5"/></svg>', 'svg'],
    ['a style tag', '<style>body{display:none}</style>', 'style'],
    ['a form', '<form action="https://evil.example"><input name="q"></form>', 'form'],
    ['video', '<video src="https://evil.example/v.mp4"></video>', 'video'],
    ['a details tag', '<details><summary>x</summary>y</details>', 'details'],
  ])('drops %s', (_name, src, tag) => {
    const html = renderMessageMarkdown(src)
    expect(dom(html).content.querySelector(tag)).toBeNull()
    expect(html).not.toContain('evil.example/p.png')
  })

  it.each([
    ['style', '<p style="color:red">x</p>'],
    ['background', '<table background="https://evil.example/b.png"><tr><td>x</td></tr></table>'],
    ['id', '<p id="app">x</p>'],
    ['name', '<a name="top" href="https://ok.example">x</a>'],
    ['class', '<p class="danger">x</p>'],
    ['ping', '<a href="https://ok.example" ping="https://evil.example">x</a>'],
    ['onclick', '<p onclick="alert(1)">x</p>'],
    ['data-', '<p data-cb-idx="0">x</p>'],
  ])('drops the %s attribute', (attr, src) => {
    const html = renderMessageMarkdown(src)
    expect(html).not.toContain(`${attr}=`)
    expect(html).not.toContain('evil.example')
  })

  it.each([
    ['javascript', '[x](javascript:alert(1))'],
    ['data', '[x](data:text/html;base64,PHNjcmlwdD4=)'],
    ['relative', '[x](/api/v1/_settings)'],
    ['dot relative', '[x](../admin)'],
    ['protocol-relative', '[x](//evil.example/x)'],
    ['raw javascript', '<a href="javascript:alert(1)">x</a>'],
    ['vbscript', '<a href="vbscript:msgbox(1)">x</a>'],
  ])('strips a %s link', (_name, src) => {
    const a = dom(renderMessageMarkdown(src)).content.querySelector('a')
    expect(a?.getAttribute('href') ?? null).toBeNull()
  })

  it.each([
    ['https', '[x](https://ok.example/a)', 'https://ok.example/a'],
    ['http', '<http://ok.example>', 'http://ok.example'],
    ['mailto', '[mail](mailto:a@ok.example)', 'mailto:a@ok.example'],
  ])('keeps an %s link and opens it apart', (_name, src, href) => {
    const a = dom(renderMessageMarkdown(src)).content.querySelector('a')!
    expect(a.getAttribute('href')).toBe(href)
    expect(a.getAttribute('rel')).toBe('noopener noreferrer nofollow')
    expect(a.getAttribute('target')).toBe('_blank')
  })

  it('links a code span naming a readable ref of the message, in the world', () => {
    const html = renderMessageMarkdown('see `TKT-1@draft` and `TKT-2`', {
      refs: [{ id: 'TKT-1@draft', type: 'ticket', title: 'Fix <b>it</b>' }],
      world: 'main',
    })
    const t = dom(html)
    const a = t.content.querySelector('a.entity-ref')!
    expect(a.getAttribute('href')).toBe('/entity/ticket/TKT-1@draft?world=main')
    expect(a.textContent).toBe('Fix <b>it</b>')
    expect(a.querySelector('b')).toBeNull()
    expect(a.getAttribute('target')).toBeNull()
    // An id the message did not resolve stays code: no lookup of its own.
    expect(t.content.querySelector('code')?.textContent).toBe('TKT-2')
  })

  it('leaves code blocks, partial matches and malformed refs alone', () => {
    const refs = [
      { id: 'TKT-1', type: 'ticket' },
      { id: 'x"><script>', type: 'ticket' },
      { id: 'TKT-3', type: 'tic ket' },
    ]
    const html = renderMessageMarkdown(
      '```\nTKT-1\n```\n\n`TKT-1 and more` `x"><script>` `TKT-3`',
      { refs }
    )
    const t = dom(html)
    expect(t.content.querySelector('a')).toBeNull()
    expect(t.content.querySelector('script')).toBeNull()
  })

  it('returns an empty string for empty content', () => {
    expect(renderMessageMarkdown('')).toBe('')
  })
})
