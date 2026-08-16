import { test, expect } from './fixtures';
import { CustomisationPage } from '../pages';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';

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

  /** Create <project>/custom/ and write a file into it. */
  const writeCustom = (projectDir: string, rel: string, body: string) => {
    const full = join(projectDir, 'custom', rel);
    mkdirSync(dirname(full), { recursive: true });
    writeFileSync(full, body);
  };

  test('operator CSS wins against a route chunk loaded after client-side navigation', async ({
    appPage,
    testProject,
  }) => {
    // A LOW-specificity operator rule (0,1,0) against rela's scoped
    // `.sidebar[data-v-hash]` (0,2,0). Before layering this lost twice over —
    // on specificity and on source order. It must now win on both.
    writeCustom(testProject, 'custom.css', `.sidebar { background-color: ${OPERATOR_GREEN}; }`);

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
    writeCustom(testProject, 'custom.js', `document.documentElement.setAttribute('data-operator-js', 'ran');`);

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
    writeCustom(
      testProject,
      'custom.js',
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

  test('an operator asset is reachable and usable from custom.css', async ({
    appPage,
    testProject,
    serverUrl,
  }) => {
    // THE POINT OF TKT-IWMETE: operator CSS can reference its own assets.
    // A Go test cannot prove the browser actually resolves url(/_custom/...),
    // so this asserts a real 200 on the wire plus the applied style.
    writeCustom(testProject, 'logo.svg', '<svg xmlns="http://www.w3.org/2000/svg"/>');
    writeCustom(testProject, 'fonts/brand.woff2', 'NOT-A-REAL-FONT');
    writeCustom(
      testProject,
      'custom.css',
      `.sidebar { background-color: ${OPERATOR_GREEN}; background-image: url(/_custom/logo.svg); }`,
    );

    const custom = new CustomisationPage(appPage);
    await custom.navigateTo('/');
    await custom.expectSidebarBackground(OPERATOR_GREEN);

    const svg = await custom.fetchCustomAsset(serverUrl, 'logo.svg');
    expect(svg.status).toBe(200);
    expect(svg.contentType).toContain('image/svg+xml');

    const font = await custom.fetchCustomAsset(serverUrl, 'fonts/brand.woff2');
    expect(font.status, 'nested asset must be reachable').toBe(200);
    expect(font.contentType).toContain('font/woff2');
  });

  test('files outside custom/ are unreachable, and dotfiles inside it are refused', async ({
    appPage,
    testProject,
    serverUrl,
  }) => {
    // INVERTED from the pre-TKT-IWMETE test: an arbitrary file INSIDE custom/
    // is now meant to serve. What must never be reachable is anything OUTSIDE
    // it, or a dot-prefixed path inside it.
    writeFileSync(join(testProject, 'outside.txt'), 'LEAKED-OUTSIDE');
    writeCustom(testProject, 'inside.txt', 'operator content');
    writeCustom(testProject, '.env', 'SECRET=1');

    const custom = new CustomisationPage(appPage);

    const inside = await custom.fetchCustomAsset(serverUrl, 'inside.txt');
    expect(inside.status, 'a plain file inside custom/ must serve').toBe(200);

    const dotfile = await custom.fetchCustomAsset(serverUrl, '.env');
    expect(dotfile.status, 'dot-prefixed entries must 404').toBe(404);
    expect(dotfile.body).not.toContain('SECRET');

    for (const p of ['../outside.txt', '..%2Foutside.txt', 'sub/../../outside.txt']) {
      const res = await custom.fetchCustomAsset(serverUrl, p);
      expect(res.body, `${p} must never serve content from outside custom/`).not.toContain(
        'LEAKED-OUTSIDE',
      );
    }
  });

  test('an unchanged asset revalidates to a 304 instead of re-transferring', async ({
    appPage,
    testProject,
    serverUrl,
  }) => {
    // The point of moving to http.ServeContent: a webfont or image is not
    // re-downloaded on every navigation. Asserted over real HTTP because this
    // is protocol behaviour a Go unit test can only approximate.
    writeCustom(testProject, 'logo.svg', '<svg xmlns="http://www.w3.org/2000/svg"/>');

    const custom = new CustomisationPage(appPage);

    const first = await custom.fetchCustomAssetWithHeaders(serverUrl, 'logo.svg');
    expect(first.status).toBe(200);
    const etag = first.headers['etag'];
    expect(etag, 'asset must carry a validator').toBeTruthy();
    expect(first.headers['cache-control']).toContain('no-cache');

    const second = await custom.fetchCustomAssetWithHeaders(serverUrl, 'logo.svg', {
      'If-None-Match': etag,
    });
    expect(second.status, 'unchanged asset must revalidate to 304').toBe(304);
    expect(second.body, '304 must carry no body').toBe('');
  });
});
