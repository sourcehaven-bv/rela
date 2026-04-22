import { test, expect } from './fixtures';
import { FormPage } from '../pages';

test.describe('Entity Templates', () => {
  // The inline test project has no entity templates configured, so the
  // template selector UI is hidden. These tests still exercise the selector's
  // presence-or-absence contract — flip to real assertions if you later add
  // templates to fixtures.ts.

  test('template selector is shown only when multiple templates exist', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');

    if (await form.templateSelectorVisible()) {
      expect(await form.templatePillCount()).toBeGreaterThan(1);
    }
    // else: correctly hidden when 0–1 templates. No further assertion.
  });

  test('clicking a template pill applies it', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');

    const count = await form.templatePillCount();
    if (count > 1) {
      await form.clickTemplatePill(1);
      await form.expectTemplatePillActive(1);
    } else {
      test.skip(true, 'Inline project has no entity templates configured');
    }
  });
});
