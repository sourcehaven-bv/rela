import { test, expect } from './fixtures';
import { FormPage } from '../pages';

test.describe('Markdown Body Editor', () => {
  const createdFeatures: string[] = [];

  test.afterEach(async ({ api }) => {
    while (createdFeatures.length) {
      const id = createdFeatures.pop()!;
      await api.deleteEntity('features', id).catch(() => {});
    }
  });

  test('create form shows the Content body field', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectContentFieldVisible();
    await form.expectContentLabelHasText('Content');
  });

  test('markdown editor renders with toolbar', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
  });

  test('markdown editor bundles Font Awesome (no CDN fetch)', async ({ appPage }) => {
    // Regression guard for TKT-ZDRS. EasyMDE's default behavior is to inject
    // <link href="https://maxcdn.bootstrapcdn.com/font-awesome/..."> into the
    // page at runtime; we pass `autoDownloadFontAwesome: false` and ship FA
    // 4.7 via npm so the binary stays self-contained.
    //
    // The "no off-origin fetch" half of this invariant is now enforced for
    // EVERY test by the appPage fixture in tests/fixtures.ts — if EasyMDE
    // ever reintroduces a CDN <link>, every test that mounts a markdown
    // editor will fail in afterEach with the offending URL. This test owns
    // the complementary assertion: that the bundled FA stylesheet actually
    // applied to the toolbar buttons. The two together form the regression
    // guard.
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();

    // EasyMDE renders its toolbar glyphs via `fa fa-*` classes that resolve
    // (only with FA loaded) to FontAwesome's icon font on the `::before`
    // pseudo-element. If our bundled CSS failed to land, the computed family
    // falls back to a generic and this assertion fails with the actual
    // family in the error message.
    const boldFontFamily = await form.getBoldToolbarIconFontFamily();
    expect(boldFontFamily, 'bold toolbar button not in DOM').not.toBeNull();
    expect(
      boldFontFamily?.toLowerCase(),
      `expected ::before font-family to include 'fontawesome', got ${boldFontFamily}`,
    ).toContain('fontawesome');
  });

  test('can fill body content and submit form', async ({ appPage, api }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');

    await form.fillField('title', 'E2E Body Test Feature');
    await form.selectField('priority', 'low');
    await form.expectMarkdownEditorReady();
    await form.typeMarkdownBody('# Test Content\n\nThis is body content.');

    const created = await form.submitAndExpectCreate('features');
    createdFeatures.push(created.id);

    const entity = await api.getEntity('features', created.id);
    expect(entity.properties.title).toBe('E2E Body Test Feature');
  });
});
