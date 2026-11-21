import { bootMeta } from './bootMeta'

function addMeta(name: string, content: string): HTMLMetaElement {
  const m = document.createElement('meta')
  m.name = name
  m.content = content
  document.head.appendChild(m)
  return m
}

describe('bootMeta (BUG-21N3J4N)', () => {
  afterEach(() => {
    document.head.querySelectorAll('meta[name^="rela-"]').forEach((m) => m.remove())
    vi.resetModules()
  })

  it('reads a meta the server wrote and null when absent or empty', () => {
    expect(bootMeta('rela-app-name')).toBeNull()
    addMeta('rela-app-name', ' Sourcehaven ')
    addMeta('rela-language', '')
    expect(bootMeta('rela-app-name')).toBe('Sourcehaven')
    expect(bootMeta('rela-language')).toBeNull()
  })

  it('makes the server language the project default before the config loads', async () => {
    addMeta('rela-language', 'en')
    const spy = vi.spyOn(navigator, 'languages', 'get').mockReturnValue(['nl-NL'])
    const { resolvedLanguage } = await import('./locale')
    // A Dutch browser, as in the report: the server's language wins.
    expect(resolvedLanguage.value).toMatchObject({ language: 'en', source: 'project' })
    spy.mockRestore()
  })
})
