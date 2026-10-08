// @vitest-environment jsdom
//
// Renders every case of the Go segments golden through the real pipeline
// (applyHighlights, then renderMarkdown with marked and DOMPurify) and checks
// the marks. The server computes segments with goldmark; this is where a case
// that goldmark and marked parse differently shows up. jsdom, not happy-dom:
// see BUG-SQSV6V in markdown.test.ts.
import { describe, it, expect } from 'vitest'
import { applyHighlights } from './commentHighlight'
import { renderMarkdown } from './markdown'
import goldens from '../../../internal/comments/testdata/segments_golden.json'

describe('server segments render as marks', () => {
  for (const g of goldens) {
    it(g.name, () => {
      const el = document.createElement('div')
      el.innerHTML = renderMarkdown(
        applyHighlights(g.body, [{ id: 'c1', start: g.start, end: g.end, segments: g.segments }])
      )
      const marks = [...el.querySelectorAll('mark[data-comment-id="c1"]')].map((m) => m.textContent)
      expect(marks).toEqual(g.marks)
      // A mark the renderer did not parse as markup shows up as text.
      expect(el.textContent).not.toContain('<mark')
      expect(el.textContent).not.toContain('</mark>')
    })
  }
})
