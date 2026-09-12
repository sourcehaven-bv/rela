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

  test('the editor ships no icon font and fetches nothing off-origin', async ({
    appPage,
  }) => {
    // Replaces a Font Awesome regression guard (TKT-ZDRS) that was specific to
    // EasyMDE: it injected a maxcdn <link> at runtime unless told not to, so
    // the old test asserted the BUNDLED font had applied to the toolbar.
    //
    // Milkdown's toolbar is inline SVG, so the stronger property now holds —
    // there is no icon font to bundle or fetch. Asserting the absence directly
    // keeps the invariant meaningful instead of deleting the coverage.
    //
    // The "nothing off-origin" half is enforced for EVERY test by the appPage
    // fixture, which fails in afterEach on any off-origin request.
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();

    // Toolbar glyphs are real <svg> elements, not glyph-bearing pseudo-elements.
    const svgCount = await form.countToolbarSvgIcons();
    expect(svgCount, 'toolbar should render inline SVG icons').toBeGreaterThan(0);

    // No stylesheet or preloaded font declares an icon font family.
    const iconFontRefs = await appPage.evaluate(() =>
      [...document.querySelectorAll('link[rel="stylesheet"], link[rel="preload"]')]
        .map((el) => el.getAttribute('href') ?? '')
        .filter((href) => /font-?awesome|fontawesome/i.test(href)),
    );
    expect(iconFontRefs, 'no Font Awesome stylesheet should be loaded').toEqual([]);
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
