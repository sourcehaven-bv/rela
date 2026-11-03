import { test, expect } from './fixtures';
import { FormPage } from '../pages';

test.describe('Entity Templates', () => {
  // The inline project seeds two feature templates (`feature.md` +
  // `feature--spike.md`), so the template dropdown renders two options and
  // these tests can assert real behaviour without per-test skip logic.

  test('template selector lists each seeded template', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');

    expect(await form.templateSelectorVisible()).toBe(true);
    expect(await form.templateOptionLabels()).toEqual(['Default', 'Spike']);
  });

  test('choosing a template selects it', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');

    await form.selectTemplate('Spike');
    await form.expectTemplateSelected('Spike');
  });
});
