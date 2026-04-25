// CommonJS flat config — e2e/package.json has no "type: module".
const js = require('@eslint/js');
const tseslint = require('typescript-eslint');

// Low-level Playwright APIs that bypass the Page Object pattern when called
// from a spec. Page objects themselves remain free to use these to encapsulate
// selectors — that's their job.
const FORBIDDEN_SELECTOR_METHODS = [
  'locator',
  'getByRole',
  'getByTestId',
  'getByText',
  'getByLabel',
  'getByPlaceholder',
  'getByTitle',
  'getByAltText',
];

module.exports = tseslint.config(
  {
    ignores: [
      'node_modules/**',
      'test-results/**',
      'playwright-report/**',
      // The config itself is CommonJS; it doesn't need TS lint rules.
      'eslint.config.js',
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    // Spec-only rules: enforce Page Object Pattern by banning raw selector,
    // timing, and unwrapped-fetch primitives. See e2e/tests/AGENTS.md for
    // rationale. Page objects are exempted below.
    files: ['tests/**/*.spec.ts'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: `CallExpression[callee.property.name=/^(${FORBIDDEN_SELECTOR_METHODS.join('|')})$/]`,
          message:
            'Use a page-object method instead of calling Playwright selectors directly from a spec. ' +
            'Extend the relevant page object in e2e/pages/ and call its method.',
        },
        {
          selector: "CallExpression[callee.property.name='waitForTimeout']",
          message:
            'waitForTimeout is flaky. Use expect.poll, locator waits, or a page-object waitForX helper instead.',
        },
        {
          // Bans `page.request.fetch(...)` / `something.request.fetch(...)`
          // from specs so they can't bypass the `api` fixture's Origin
          // injection. See RR-3VPYE.
          selector:
            "CallExpression[callee.object.property.name='request'][callee.property.name='fetch']",
          message:
            'Do not call request.fetch directly in specs — use the `api` fixture so the Origin header is set consistently. ' +
            'If you specifically want to test missing-Origin rejection, use a dedicated origin-security spec that bypasses `api`.',
        },
        {
          // Bans `api.rawRequest(...)` from spec files. The typed helpers
          // (createEntity, getEntity, listRelations, updateEntity, …) cover
          // legitimate seed and verification needs. rawRequest is the un-typed
          // escape hatch — reaching for it in spec code means you're testing
          // HTTP API shape, which belongs in a Go integration test under
          // internal/dataentry/. Same scope as the `request.fetch` ban above:
          // anywhere in a spec, not just inside `test(...)` bodies. The
          // fixture itself is exempted via the `tests/fixtures.ts` relax block.
          selector:
            "CallExpression[callee.object.name='api'][callee.property.name='rawRequest']",
          message:
            'Do not call api.rawRequest in spec code — that is HTTP-shape testing, which belongs in a Go integration test under internal/dataentry/. ' +
            'For UI tests use the typed api helpers (createEntity, getEntity, listRelations, updateEntity) for seed and verify.',
        },
        {
          // Same ban for bracket-access — `api['rawRequest'](...)` — so the
          // dotted form isn't trivially circumvented.
          selector:
            "CallExpression[callee.object.name='api'][callee.computed=true][callee.property.value='rawRequest']",
          message:
            "Do not call api['rawRequest'] in spec code — that is HTTP-shape testing, which belongs in a Go integration test under internal/dataentry/. " +
            'For UI tests use the typed api helpers (createEntity, getEntity, listRelations, updateEntity) for seed and verify.',
        },
      ],
    },
  },
  {
    // Relax rules for test-support code that legitimately needs selectors.
    files: ['pages/**/*.ts', 'tests/fixtures.ts'],
    rules: {
      'no-restricted-syntax': 'off',
    },
  },
  {
    rules: {
      // Fixtures use `async ({}, use) => ...` — Playwright's destructuring shape.
      '@typescript-eslint/no-empty-object-type': 'off',
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
    },
  },
);
