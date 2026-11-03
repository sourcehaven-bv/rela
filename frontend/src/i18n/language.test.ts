import { resolveLanguage, type LanguageInputs, type ResolvedLanguage } from './language'

describe('resolveLanguage', () => {
  const cases: { name: string; inputs: LanguageInputs; want: ResolvedLanguage }[] = [
    {
      name: 'nothing set falls back to English',
      inputs: { browser: [] },
      want: { language: 'en', locale: 'en', source: 'fallback' },
    },
    {
      name: 'a Dutch browser gets Dutch',
      inputs: { browser: ['nl-NL', 'en'] },
      want: { language: 'nl', locale: 'nl-NL', source: 'browser' },
    },
    {
      name: 'an English browser gets English with its region',
      inputs: { browser: ['en-US'] },
      want: { language: 'en', locale: 'en-US', source: 'browser' },
    },
    {
      name: 'an unsupported first browser language is skipped',
      inputs: { browser: ['de-DE', 'nl'] },
      want: { language: 'nl', locale: 'nl', source: 'browser' },
    },
    {
      name: 'only unsupported browser languages fall back to English text, browser formatting',
      inputs: { browser: ['de-DE', 'fr'] },
      want: { language: 'en', locale: 'de-DE', source: 'fallback' },
    },
    {
      name: 'the project default wins over the browser',
      inputs: { project: 'nl', browser: ['en-GB'] },
      want: { language: 'nl', locale: 'nl', source: 'project' },
    },
    {
      name: 'a project region is kept for formatting',
      inputs: { project: 'nl-BE', browser: ['en-US'] },
      want: { language: 'nl', locale: 'nl-BE', source: 'project' },
    },
    {
      name: 'the user choice wins over the project default',
      inputs: { user: 'en', project: 'nl', browser: ['nl-NL'] },
      want: { language: 'en', locale: 'en', source: 'user' },
    },
    {
      name: 'a bare user choice borrows the browser region of the same language',
      inputs: { user: 'en', browser: ['nl-NL', 'en-GB'] },
      want: { language: 'en', locale: 'en-GB', source: 'user' },
    },
    {
      name: 'an unsupported user value is ignored',
      inputs: { user: 'fr', project: 'nl', browser: [] },
      want: { language: 'nl', locale: 'nl', source: 'project' },
    },
    {
      name: 'an invalid project tag is ignored',
      inputs: { project: 'dutch!', browser: ['nl-NL'] },
      want: { language: 'nl', locale: 'nl-NL', source: 'browser' },
    },
    {
      name: 'case is normalised',
      inputs: { project: 'NL-be', browser: [] },
      want: { language: 'nl', locale: 'nl-BE', source: 'project' },
    },
    {
      name: 'an invalid browser entry is skipped',
      inputs: { browser: ['', 'x_y', 'nl'] },
      want: { language: 'nl', locale: 'nl', source: 'browser' },
    },
  ]

  it.each(cases)('$name', ({ inputs, want }) => {
    expect(resolveLanguage(inputs)).toEqual(want)
  })
})
