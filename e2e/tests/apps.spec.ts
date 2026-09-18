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

/** The optional <rela-editor> element, running inside the app's sandbox.
 *
 * These are the assertions that cannot be made anywhere else. Unit tests mount
 * the element under happy-dom, which neither lays anything out nor enforces a
 * CSP — so "the served stylesheet applied" and "ProseMirror's own DOM writes are
 * not blocked" are only answerable in a real browser under the real path-scoped
 * header (TKT-D2JML7).
 */
test.describe('Custom apps: the embedded markdown editor', () => {
  test('mounts and reports the markdown it was given, unchanged', async ({ appPage }) => {
    // The write-back guard sits on `.value`: a WYSIWYG round-trip reformats, and
    // an app that only displayed a body must get its own bytes back.
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');
    await app.waitForEditor();
    expect(await app.editorMarkdown()).toBe(
      '# Title\n\n- one\n- two\n\nSee `FEAT-001` for detail.\n\n| a | b |\n| - | - |\n| 1 | 2 |\n',
    );
  });

  test('the served stylesheet actually applied', async ({ appPage }) => {
    // The app CSP has no 'unsafe-inline', so the editor links a FILE rather than
    // injecting a <style>. If that link were ever swapped back for an injected
    // element the rules would be dropped silently — the element lands in the DOM
    // and its .sheet stays null — and the editor would render unstyled with only
    // a console violation to show for it.
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');
    await app.waitForEditor();
    // A rule that exists only in the served stylesheet.
    expect(await app.editorShellStyle('position')).toBe('relative');
    // And one from markdown-content.css, which the bundle concatenates so the
    // writing surface is styled by the same rules that render the body later.
    expect(await app.editorSurfaceStyle('overflow-wrap')).toBe('anywhere');
  });

  test('draws its toolbar with inline SVG, shipping no webfont', async ({ appPage }) => {
    // The EasyMDE build served a Font Awesome woff2 at a third reserved path,
    // with a CORS exception because the sandboxed iframe is null-origin. Every
    // glyph is now inline SVG, so a missing one is a blank button rather than a
    // tofu box, and that path is gone.
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');
    await app.waitForEditor();
    const buttons = await app.editorToolbarButtonCount();
    expect(buttons).toBeGreaterThan(8);
    expect(await app.editorToolbarIconCount()).toBe(buttons);
  });

  test('the old webfont path is gone', async ({ appPage, serverUrl }) => {
    const res = await appPage.request.get(`${serverUrl}/api/v1/_apps/e2e-demo/_rela-editor.woff2`, {
      headers: { Origin: serverUrl },
    });
    expect(res.status()).toBe(404);
  });

  test('renders an entity reference as its bare ID', async ({ appPage }) => {
    // The app bridge has no per-principal mentions endpoint, so there is no title
    // the editor may show without routing around the read gate (BUG-R9EHKV). The
    // ID is the correct degraded state, not an error.
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');
    await app.waitForEditor();
    expect(await app.editorEntityRefs()).toEqual(['FEAT-001']);
  });

  test('formats through the toolbar and reports the result on .value', async ({ appPage }) => {
    // ProseMirror positions its chrome by writing DOM style PROPERTIES, which
    // `style-src` permits (it governs stylesheets and the style ATTRIBUTE). A
    // command that runs end to end here is the evidence that nothing in the
    // editing path is CSP-blocked.
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');
    await app.waitForEditor();
    // The body opens on its heading, so the toolbar should say so before
    // anything is pressed.
    await app.clickEditorLine('Title');
    expect(await app.editorCommandActive('h1')).toBe(true);
    // Pressing an active block command runs its inverse, so the button toggles
    // rather than no-opping on a block that is already that type.
    await app.clickEditorCommand('h1');
    expect(await app.editorMarkdown()).toContain('Title');
    expect(await app.editorMarkdown()).not.toContain('# Title');
  });

  test('disables a command that would do nothing where the cursor is', async ({ appPage }) => {
    // A heading cannot be applied inside a list item. The disabled state is a DRY
    // RUN of the command itself, so it cannot drift from what the command does —
    // and `aria-disabled` keeps the button focusable, so moving the cursor never
    // drops focus to <body> mid-interaction.
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');
    await app.waitForEditor();
    await app.clickEditorLine('one');
    expect(await app.editorCommandActive('bulletList')).toBe(true);
    expect(await app.editorCommandUnavailable('h1')).toBe(true);
    expect(await app.editorCommandHasNativeDisabled('h1')).toBe(false);
  });

  test('raises no CSP violation while the editor runs', async ({ appPage }) => {
    // The recorded blocker for this port claimed `style-src` without
    // 'unsafe-inline' would break ProseMirror. It does not, and this is what says
    // so: a violation anywhere in editing fails here.
    const app = new AppHostPage(appPage);
    await app.open('e2e-demo');
    await app.waitForEditor();

    // Listening starts only NOW, after load. The demo app deliberately provokes
    // three CSP violations on startup (its own csp-probe reaching for /api/, which
    // the test above asserts), and filtering those out by matching a seeded entity
    // id would leave this test red whenever that probe's target changed — for a
    // reason having nothing to do with the editor, which is how a test that proves
    // something becomes one that gets widened until it proves nothing. Scoping the
    // window is what makes every violation seen here the editor's.
    const violations: string[] = [];
    appPage.on('console', (msg) => {
      const t = msg.text();
      if (/Content Security Policy|Refused to/i.test(t)) violations.push(t);
    });

    await app.clickEditorLine('for detail');
    await app.clickEditorCommand('bulletList');
    // A list marker, whichever one remark picks: it alternates `-` and `*`
    // between adjacent lists so two of them cannot merge into one.
    expect(await app.editorMarkdown()).toMatch(/^[-*] See `FEAT-001`/m);
    expect(violations).toEqual([]);
  });
});
