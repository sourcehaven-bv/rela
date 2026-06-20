import { test, expect } from './fixtures';
import { AppHostPage } from '../pages';
import { SEED } from './fixtures';

test.describe('Custom apps (sandboxed-iframe bridge)', () => {
  test('app reads graph data through the bridge on load', async ({ appPage, api }) => {
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');

    // The app called rela.list({type:'feature'}) over the MessageChannel and
    // rendered the count. Cross-check against the REST API directly.
    const features = await api.listEntities('features');
    await app.expectFeatureCount(features.meta.total);
  });

  test('the CSP actually blocks the app from reaching /api/ directly', async ({ appPage }) => {
    // The boundary is the path-scoped CSP, not origin isolation. This drives a
    // real browser: the app's own JS tries fetch('/api/...') (connect-src 'none')
    // and an <img src=/api/...> (path-scoped img-src). Both must be blocked —
    // asserting the header *string* (done in unit tests) isn't the same as the
    // browser enforcing it. Guards against a future regression to 'self'.
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');
    await expect(app.cspProbe).toHaveText('blocked');
  });

  test('iframe is sandboxed without allow-same-origin', async ({ appPage }) => {
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');

    const sandbox = await app.iframeSandbox();
    expect(sandbox).toContain('allow-scripts');
    // The load-bearing isolation guarantee: never allow-same-origin, so the
    // app stays origin-"null" and cannot reach /api/ directly.
    expect(sandbox).not.toContain('allow-same-origin');
  });

  test('app writes a relation through the bridge (re-authorized by the server)', async ({
    appPage,
    api,
  }) => {
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');

    // Precondition: no blocks edge yet between the two seeded features.
    const before = await api.listRelations('features', SEED.features.authentication, 'blocks');
    expect(before.map((e) => e.id)).not.toContain(SEED.features.dashboardAnalytics);

    await app.clickLink();

    // The bridge write went through the normal entitymanager path, so the edge
    // is now persisted and visible via the REST API.
    const after = await api.listRelations('features', SEED.features.authentication, 'blocks');
    expect(after.map((e) => e.id)).toContain(SEED.features.dashboardAnalytics);
  });

  test('a cross-origin write to a bridge target is rejected (same-origin guard)', async ({
    appPage,
    serverUrl,
  }) => {
    // The host page makes same-origin calls; a forged cross-origin POST must be
    // rejected by requireSameOrigin. page.request shares the context's default
    // Origin header (the server's own origin), so override it with a foreign
    // Origin to simulate a malicious cross-site caller.
    const resp = await appPage.request.post(
      `${serverUrl}/api/v1/features/${SEED.features.authentication}/relations/blocks`,
      {
        data: { id: SEED.features.exportData },
        headers: { Origin: 'https://evil.example' },
      },
    );
    expect(resp.status()).toBe(403);
  });

  test('unknown app id returns 404', async ({ appPage, serverUrl }) => {
    const resp = await appPage.request.get(`${serverUrl}/api/v1/_apps/does-not-exist/`);
    expect(resp.status()).toBe(404);
  });

  test('app serves sibling assets but the CSP confines them', async ({ appPage, serverUrl }) => {
    // The index loads from a real URL (so multi-file apps work); the response
    // carries a path-scoped CSP header (not a <meta>), with connect-src 'none'
    // so the app's own JS cannot reach /api/ — only the bridge can.
    const resp = await appPage.request.get(`${serverUrl}/api/v1/_apps/e2e-demo/`);
    expect(resp.status()).toBe(200);
    const csp = resp.headers()['content-security-policy'] ?? '';
    expect(csp).toContain('/api/v1/_apps/e2e-demo/');
    expect(csp).toContain("connect-src 'none'");
    // The script-src source MUST be an absolute scheme://host/... URL, not a
    // bare path — a path-only CSP source is invalid and browsers ignore it,
    // silently blocking the app's own scripts (a regression that would only
    // otherwise surface as an opaque bridge timeout).
    expect(csp).toMatch(/script-src\s+https?:\/\/[^/]+\/api\/v1\/_apps\/e2e-demo\//);
    // The SDK is served from the app's own reserved path.
    const sdk = await appPage.request.get(`${serverUrl}/api/v1/_apps/e2e-demo/_rela.js`);
    expect(sdk.status()).toBe(200);
    expect(await sdk.text()).toContain('window.rela');
  });

  test('serves the optional _rela.css (theme tokens + base controls)', async ({
    appPage,
    serverUrl,
  }) => {
    const resp = await appPage.request.get(`${serverUrl}/api/v1/_apps/e2e-demo/_rela.css`);
    expect(resp.status()).toBe(200);
    expect(resp.headers()['content-type']).toContain('css');
    const css = await resp.text();
    expect(css).toContain('--text-color'); // theme tokens
    expect(css).toContain(':root.dark'); // dark variant
    expect(css).toContain('.btn'); // base controls
    expect(css).toContain('.input');
    expect(css).toContain('.card');
  });
});
