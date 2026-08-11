import { test, expect } from './fixtures';
import { CustomisationPage } from '../pages';
import { writeFileSync } from 'node:fs';
import { join } from 'node:path';

/**
 * Operator customisation hooks — TKT-3DBK6I.
 *
 * These tests exist because the ORIGINAL design premise was wrong in a way
 * static analysis could not catch. The proposal assumed an operator <link> in
 * <head> would win the cascade. In a real production build it does not: Vite
 * emits ~19 stylesheets, and the 18 route-level chunks are appended to <head>
 * at RUNTIME (`__vitePreload` → `document.head.appendChild`), i.e. AFTER the
 * injected operator link. At equal specificity the later sheet won, so a skin
 * worked on the dashboard and silently died on a list view.
 *
 * The fix is `@layer rela`: an unlayered declaration outranks a layered one
 * regardless of order OR specificity. Only a browser can verify that, which is
 * why this is an e2e test and not a Go assertion about build output. Asserting
 * that stylesheets are layer-wrapped verifies the MECHANISM; this verifies the
 * PROPERTY.
 */
test.describe('Operator customisation hooks', () => {
  const OPERATOR_GREEN = 'rgb(0, 255, 0)';

  test('operator CSS wins against a route chunk loaded after client-side navigation', async ({
    appPage,
    testProject,
  }) => {
    // A LOW-specificity operator rule (0,1,0) against rela's scoped
    // `.sidebar[data-v-hash]` (0,2,0). Before layering this lost twice over —
    // on specificity and on source order. It must now win on both.
    writeFileSync(join(testProject, 'custom.css'), `.sidebar { background-color: ${OPERATOR_GREEN}; }`);

    const custom = new CustomisationPage(appPage);
    await custom.navigateTo('/');
    expect(await custom.hasCustomStylesheet()).toBe(true);
    await custom.expectSidebarBackground(OPERATOR_GREEN);

    // Now the case that actually broke: navigate CLIENT-SIDE so a route-level
    // CSS chunk is appended to <head> after the operator's stylesheet.
    const before = (await custom.stylesheetHrefs()).length;
    await custom.navigateTo('/list/features');
    await expect.poll(async () => (await custom.stylesheetHrefs()).length).toBeGreaterThan(before);

    // The operator rule must still win now that a later sheet exists.
    await custom.expectSidebarBackground(OPERATOR_GREEN);
  });

  test('stock deployment injects nothing', async ({ appPage }) => {
    // No custom.css / custom.js in the project → no references in the shell.
    const custom = new CustomisationPage(appPage);
    await custom.navigateTo('/');

    expect(await custom.hasCustomStylesheet()).toBe(false);
    expect(await custom.hasCustomScript()).toBe(false);
  });

  test('custom.js is served as a module and executes', async ({ appPage, testProject }) => {
    // Deliberately does NOT assume the SPA has rendered. type="module" defers
    // past DOM parse, but Vue mounts asynchronously AFTER that — measured:
    // querying '.sidebar' at module time returns null even though the sidebar
    // exists moments later. That is the documented gotcha (operator code must
    // wait for what it needs, e.g. via MutationObserver), so the test pins
    // "the module executed", not "the app was ready".
    writeFileSync(
      join(testProject, 'custom.js'),
      `document.documentElement.setAttribute('data-operator-js', 'ran');`,
    );

    const custom = new CustomisationPage(appPage);
    await custom.navigateTo('/');

    expect(await custom.hasCustomScript()).toBe(true);
    await custom.expectOperatorScriptMarker('data-operator-js', 'ran');
  });

  test('operator JS can await the SPA rendering (the documented gotcha)', async ({
    appPage,
    testProject,
  }) => {
    // The supported way to touch rela's DOM: observe rather than assume. This
    // pins the workaround the docs prescribe, so it cannot silently rot.
    writeFileSync(
      join(testProject, 'custom.js'),
      `const mark = () => {
         if (!document.querySelector('.sidebar')) return false;
         document.documentElement.setAttribute('data-operator-js', 'sidebar-seen');
         return true;
       };
       if (!mark()) {
         const obs = new MutationObserver(() => { if (mark()) obs.disconnect(); });
         obs.observe(document.body, { childList: true, subtree: true });
       }`,
    );

    const custom = new CustomisationPage(appPage);
    await custom.navigateTo('/');
    await custom.expectOperatorScriptMarker('data-operator-js', 'sidebar-seen');
  });

  test('a non-allowlisted project file is not served under /_custom/', async ({
    appPage,
    testProject,
    serverUrl,
  }) => {
    writeFileSync(join(testProject, 'secret.txt'), 'TOPSECRET');

    // A non-allowlisted name under the prefix is a flat 404.
    const direct = await appPage.request.get(`${serverUrl}/_custom/secret.txt`);
    expect(direct.status()).toBe(404);
    expect(await direct.text()).not.toContain('TOPSECRET');

    // Traversal spellings must never yield the file. The STATUS varies by
    // spelling — Go's ServeMux normalises `/_custom/../secret.txt` to
    // `/secret.txt` and 307s, which then falls through to the SPA shell — so
    // the invariant worth pinning is the absence of the content, not a
    // particular code. Asserting 404 here would encode mux normalisation
    // behaviour rather than the security property.
    for (const path of [
      '/_custom/../secret.txt',
      '/_custom/%2e%2e%2fsecret.txt',
      '/_custom/..%2Fsecret.txt',
      '/_custom/custom.css/../secret.txt',
    ]) {
      const res = await appPage.request.get(`${serverUrl}${path}`);
      expect(await res.text(), `${path} must never serve project-root content`).not.toContain(
        'TOPSECRET',
      );
    }
  });
});
