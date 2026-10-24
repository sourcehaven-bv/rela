// Browser half of seqtrace. Playwright injects this before the SPA loads
// (RELA_E2E_SEQTRACE, see e2e/tests/fixtures.ts); the SPA itself has no
// tracing code. For every same-origin /api/ request it records the JavaScript
// stack at the moment of the call and sends a span id in the X-Seqtrace-Span
// header. The traced server records that id on the request's root call, and
// `seqtrace stitch` puts the server's call tree under the browser's.
//
// Each record goes to the test runner as it is made, through the
// window.__relaSeqtracePush binding the fixture exposes, so a navigation or a
// second page loses nothing. Only the method, the path and the stack are
// kept: no bodies, no query strings, no headers.
;(() => {
  if (window.__relaSeqtrace || typeof window.__relaSeqtracePush !== 'function') return
  // V8 keeps 10 frames by default; axios and this file can use them all
  // before the stack reaches the SPA.
  Error.stackTraceLimit = 64
  const push = window.__relaSeqtracePush
  // Ids must be unique across every document of a test, so each document
  // draws its own prefix.
  const page = Math.random().toString(36).slice(2, 10)
  let next = 0
  Object.defineProperty(window, '__relaSeqtrace', { value: true })

  // span returns the id for a request to url, or "" when it is not traced.
  function span(method, url) {
    let u
    try {
      u = new URL(url, location.href)
    } catch {
      return ''
    }
    if (u.origin !== location.origin || !u.pathname.startsWith('/api/')) return ''
    const id = `${page}-${++next}`
    push({ span: id, method: String(method || 'GET').toUpperCase(), path: u.pathname, stack: new Error().stack })
    return id
  }

  const fetch0 = window.fetch
  window.fetch = function (input, init) {
    const req = input instanceof Request ? input : null
    const id = span(init?.method || req?.method, req ? req.url : String(input))
    if (!id) return fetch0.call(this, input, init)
    const headers = new Headers(init?.headers || req?.headers)
    headers.set('X-Seqtrace-Span', id)
    return fetch0.call(this, input, { ...init, headers })
  }

  const open0 = XMLHttpRequest.prototype.open
  const send0 = XMLHttpRequest.prototype.send
  XMLHttpRequest.prototype.open = function (method, url, ...rest) {
    this.__seqtraceCall = [method, url]
    return open0.call(this, method, url, ...rest)
  }
  XMLHttpRequest.prototype.send = function (body) {
    const [method, url] = this.__seqtraceCall || []
    const id = method ? span(method, url) : ''
    if (id) this.setRequestHeader('X-Seqtrace-Span', id)
    return send0.call(this, body)
  }
})()
