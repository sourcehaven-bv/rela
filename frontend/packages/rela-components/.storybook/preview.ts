import type { Preview } from '@storybook/vue3-vite'
import '../src/styles/base.css'

/** Matches the breakpoints in src/styles/tokens.css. */
const viewports = {
  phone: {
    name: 'Phone (compact)',
    styles: { width: '390px', height: '844px' },
    type: 'mobile' as const,
  },
  tablet: {
    name: 'Tablet (medium)',
    styles: { width: '900px', height: '1000px' },
    type: 'tablet' as const,
  },
  desktop: {
    name: 'Desktop (wide)',
    styles: { width: '1440px', height: '1024px' },
    type: 'desktop' as const,
  },
}

/*
 * Theme switcher. Writes the class the library reads onto `<html>` rather
 * than wrapping the story in a themed `<div>`, because menus, tooltips and
 * modals teleport to `<body>` and would otherwise keep the light palette
 * while the story behind them went dark.
 *
 * `system` clears both classes and lets `prefers-color-scheme` decide.
 */
const applyTheme = (theme: string) => {
  const root = document.documentElement
  root.classList.toggle('dark', theme === 'dark')
  root.classList.toggle('light', theme === 'light')
}

const preview: Preview = {
  globalTypes: {
    theme: {
      description: 'Colour theme',
      toolbar: {
        title: 'Theme',
        icon: 'paintbrush',
        items: [
          { value: 'system', title: 'System' },
          { value: 'light', title: 'Light' },
          { value: 'dark', title: 'Dark' },
        ],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { theme: 'light' },
  decorators: [
    (story, context) => {
      applyTheme(context.globals.theme)
      return story()
    },
  ],
  parameters: {
    layout: 'fullscreen',
    viewport: { options: viewports },
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    a11y: { test: 'todo' },
  },
}

export default preview
