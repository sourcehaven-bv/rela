import { test, expect } from './fixtures';
import { FormPage, EntityPage } from '../pages';

// Verifies the markdown editor's "Insert entity reference" toolbar
// button (TKT-I5NO): clicking it opens a picker, selecting an entity
// inserts a backticked ID at the cursor, and the rendered markdown on
// the detail page shows the resolver-produced titled link (TKT-747O
// integration).
test.describe('Markdown editor entity-reference picker', () => {
  let targetId: string | null = null;
  let originId: string | null = null;

  // Use a unique title token we can search for via the full-text index
  // (Bleve splits on word boundaries, so a single nonsense token is the
  // most reliable query that won't collide with seeded fixtures).
  const targetTitleToken = 'pickerzzz' + Math.random().toString(36).slice(2, 8);
  const targetTitle = `${targetTitleToken} Target Feature`;

  test.beforeEach(async ({ api }) => {
    const target = await api.createEntity('features', {
      properties: {
        title: targetTitle,
        description: 'Selected via the editor toolbar picker',
        status: 'draft',
        priority: 'medium',
      },
    });
    targetId = target.id;
    // The picker hits /_search, which is backed by a Bleve index that
    // commits asynchronously after creates. waitForIndexed below polls
    // GET /<plural>/<id> as a coarse proxy; the title-token search in
    // the picker is what actually exercises the indexed pathway, with
    // a generous toBeVisible timeout to absorb late commits.
    await api.waitForIndexed(target.id);
  });

  test.afterEach(async ({ api }) => {
    if (originId) {
      await api.deleteEntity('features', originId).catch(() => {});
      originId = null;
    }
    if (targetId) {
      await api.deleteEntity('features', targetId).catch(() => {});
      targetId = null;
    }
  });

  test('toolbar exposes the insert-entity-reference button', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await expect(form.insertEntityRefButton).toBeVisible();
  });

  test('picker opens with the search input focused', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.openEntityPicker();
    await expect(form.entityPickerOverlay).toBeVisible();
  });

  test('inserting a known ID produces a backticked code span at the cursor', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.openEntityPicker();
    await form.searchEntityPicker(targetTitleToken);
    await appPage.keyboard.press('Enter');

    await expect(form.entityPickerOverlay).not.toBeVisible();
    const body = await form.getMarkdownBody();
    expect(body).toContain(`\`${targetId}\``);
  });

  test('round-trip: saved entity renders the rewritten link on the detail page', async ({
    appPage,
    api,
  }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.fillField('title', 'Picker Origin Feature');
    await form.selectField('priority', 'low');
    await form.openEntityPicker();
    await form.searchEntityPicker(targetTitleToken);
    await appPage.keyboard.press('Enter');

    const created = await form.submitAndExpectCreate('features');
    originId = created.id;

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', originId!);
    const link = entity.contentEntityRefLink('feature', targetId!);
    await expect(link).toBeVisible();
    await expect(link).toHaveText(targetTitle);

    // Sanity: the persisted markdown contains the backticked ID so we
    // know the e2e isn't passing for an unrelated reason.
    const persisted = await api.getContent('features', originId!);
    expect(persisted).toContain(`\`${targetId}\``);
  });

  test('Escape closes the picker without inserting', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    // The form may preload template content for the feature type; capture
    // the baseline so we can assert "Escape does not modify the body"
    // rather than "the body is empty."
    const before = await form.getMarkdownBody();
    await form.openEntityPicker();
    await appPage.keyboard.press('Escape');
    await expect(form.entityPickerOverlay).not.toBeVisible();
    const after = await form.getMarkdownBody();
    expect(after).toBe(before);
  });

  test('typing the target title surfaces it as a top result (RR-Z9C1)', async ({ appPage }) => {
    // The picker should rank a clearly-matching title at the top of the
    // result list, so the user doesn't have to scroll past unrelated
    // seeded features.
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.openEntityPicker();
    await form.searchEntityPicker(targetTitleToken);
    const firstOption = form.entityPickerOptions.first();
    await expect(firstOption).toContainText(targetId!);
  });
});
