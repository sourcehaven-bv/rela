import { test, expect } from './fixtures';
import { FormPage } from '../pages';

test.describe('Entity Templates', () => {
  // The inline project seeds two feature templates (`feature.md` +
  // `feature--spike.md`), so the pill selector renders two pills and these
  // tests can assert real behaviour without per-test skip logic.

  test('template selector shows a pill per seeded template', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');

    expect(await form.templateSelectorVisible()).toBe(true);
    expect(await form.templatePillCount()).toBe(2);
  });

  test('clicking a template pill marks it active', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');

    await form.clickTemplatePill(1);
    await form.expectTemplatePillActive(1);
  });
});
