import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import pluginVue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'
import prettier from 'eslint-config-prettier'
import globals from 'globals'

export default tseslint.config(
  // Base JS rules
  js.configs.recommended,

  // TypeScript rules
  ...tseslint.configs.recommended,

  // Vue 3 recommended rules
  ...pluginVue.configs['flat/recommended'],

  // Prettier - disables conflicting rules
  prettier,

  // Global ignores
  {
    ignores: ['dist/**', 'node_modules/**', '*.d.ts'],
  },

  // Browser globals for app source (document, console, HTMLElement, etc.)
  // Applied to .vue and .ts files except config files (which get Node globals).
  {
    files: ['**/*.vue', '**/*.ts', '**/*.tsx'],
    languageOptions: {
      globals: {
        ...globals.browser,
        // Compile-time flag injected by vite `define` (see vite.config.ts /
        // vite-env.d.ts). Declared here so no-undef recognises it (#890).
        __E2E_TEST_HOOKS__: 'readonly',
      },
    },
  },

  // Vue file configuration
  {
    files: ['**/*.vue'],
    languageOptions: {
      parser: vueParser,
      parserOptions: {
        parser: tseslint.parser,
        ecmaVersion: 'latest',
        sourceType: 'module',
      },
    },
  },

  // TypeScript files
  {
    files: ['**/*.ts', '**/*.tsx'],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
      },
    },
  },

  // Project-specific rules
  {
    rules: {
      // Vue best practices
      'vue/multi-word-component-names': 'off', // Allow single-word component names
      'vue/no-unused-vars': 'error',
      'vue/require-default-prop': 'off', // TypeScript handles this
      'vue/require-prop-types': 'off', // TypeScript handles this
      'vue/prop-name-casing': ['error', 'camelCase'],
      'vue/component-name-in-template-casing': ['error', 'PascalCase'],
      'vue/html-self-closing': [
        'error',
        {
          html: { void: 'always', normal: 'always', component: 'always' },
          svg: 'always',
          math: 'always',
        },
      ],
      'vue/attributes-order': 'error',
      'vue/order-in-components': 'error',

      // Security: Warn on v-html usage (XSS risk)
      'vue/no-v-html': 'warn',

      // TypeScript
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      '@typescript-eslint/explicit-function-return-type': 'off',
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-non-null-assertion': 'warn',
      // Prefer type guards over type assertions
      '@typescript-eslint/consistent-type-assertions': [
        'warn',
        {
          assertionStyle: 'as',
          objectLiteralTypeAssertions: 'allow-as-parameter',
        },
      ],

      // General
      'no-console': ['warn', { allow: ['warn', 'error'] }],
      'no-debugger': 'warn',
      'prefer-const': 'error',
      'no-var': 'error',
    },
  },

  // Large file warnings (catches god components)
  {
    files: ['**/*.vue'],
    rules: {
      'max-lines': ['warn', { max: 500, skipBlankLines: true, skipComments: true }],
    },
  },

  // Ban reading `.properties.title` for display. The backend serializes a
  // metamodel-aware `_title` (honoring each type's `display_property`);
  // reading the literal `title` property only works when display_property
  // happens to be `title` and otherwise renders bare IDs (BUG-1P88YM).
  // Use entityDisplayTitle() from @/utils/entityDisplay instead. The helper
  // and its test are exempt (they own the `properties.title` reference).
  {
    files: ['**/*.vue', '**/*.ts', '**/*.tsx'],
    ignores: ['src/utils/entityDisplay.ts', '**/*.test.ts', '**/*.spec.ts'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector:
            "MemberExpression[property.name='title'] > MemberExpression[property.name='properties']",
          message:
            'Do not read `.properties.title` for display — it shows bare IDs when display_property is not "title" (BUG-1P88YM). Use entityDisplayTitle() from @/utils/entityDisplay.',
        },
        {
          selector:
            "MemberExpression[computed=true][property.value='title'] > MemberExpression[property.name='properties']",
          message:
            "Do not read `.properties['title']` for display — use entityDisplayTitle() from @/utils/entityDisplay (BUG-1P88YM).",
        },
      ],
    },
  },

  // Test files - relaxed rules
  {
    files: ['**/*.test.ts', '**/*.spec.ts', '**/test/**/*.ts'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-non-null-assertion': 'off',
      'no-console': 'off',
    },
  },

  // E2E test files - further relaxed (Playwright patterns)
  {
    files: ['e2e/**/*.ts'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-non-null-assertion': 'off',
      'no-console': 'off',
    },
  },

  // Config files and Node scripts - Node environment (vite.config.js etc.)
  {
    files: ['*.config.js', '*.config.ts', 'scripts/**/*.js'],
    languageOptions: {
      globals: {
        ...globals.node,
      },
    },
    rules: {
      'no-var': 'off', // Allow var in config files for compatibility
    },
  }
)
