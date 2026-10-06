import { test, expect } from './fixtures';
import { EntityPage } from '../pages';

/**
 * The entity body edits in place, but only from its edit button. A click on
 * the text is a reading gesture: it places a caret, selects a word or a
 * paragraph, and never opens the editor (TKT-J326ZK).
 */

const PARAGRAPHS = Array.from(
  { length: 40 },
  (_, i) => `Paragraph ${i + 1} of a body long enough to scroll well past the viewport.`,
);

test.describe('Body edit button', () => {
  test('clicking the body text selects it and does not open the editor', async ({
    appPage,
    api,
  }) => {
    const created = await api.createEntity('features', {
      properties: { title: 'Selectable body' },
      content: 'First paragraph of the body.\n\nSecond paragraph of the body.',
    });
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', created.id);

    await entity.clickBodyText('Second paragraph of the body.', 1);
    await entity.clickBodyText('Second paragraph of the body.', 2);
    expect(await entity.selectedText()).not.toBe('');
    await entity.clickBodyText('Second paragraph of the body.', 3);
    expect(await entity.selectedText()).toContain('Second paragraph of the body.');
    // The read view is swapped out in the same tick an edit starts, unlike
    // the editor, which mounts later; so its presence is the reliable check.
    await expect(entity.bodyReadView).toBeVisible();
    await expect(entity.bodyEditor).toHaveCount(0);

    await entity.contentBody.hover();
    await entity.bodyEditButton.click();
    await expect(entity.bodyEditor).toBeVisible();
  });

  test('the edit button stays in view halfway down a long body', async ({ appPage, api }) => {
    const created = await api.createEntity('features', {
      properties: { title: 'Long body' },
      content: PARAGRAPHS.join('\n\n'),
    });
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', created.id);
    await expect(entity.contentBody).toContainText('Paragraph 40');

    await entity.scrollBodyHalfway();
    await expect.poll(() => entity.bodyEditButtonInView()).toBe(true);

    await entity.bodyEditButton.click();
    await expect(entity.bodyEditor).toBeVisible();
  });

  test('on a phone the edit button stays below the sticky back bar', async ({ appPage, api }) => {
    await appPage.setViewportSize({ width: 390, height: 800 });
    const created = await api.createEntity('features', {
      properties: { title: 'Long body on a phone' },
      content: PARAGRAPHS.join('\n\n'),
    });
    const entity = new EntityPage(appPage);
    await entity.navigateToEntityFrom('feature', created.id, '/');
    await expect(entity.mobileTopbar).toBeVisible();

    await entity.scrollBodyHalfway();
    await expect.poll(() => entity.bodyEditButtonInView()).toBe(true);
    await expect.poll(() => entity.bodyEditButtonClearsTopbar()).toBe(true);
  });
});
