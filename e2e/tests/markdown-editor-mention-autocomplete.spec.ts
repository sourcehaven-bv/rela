import { test, expect } from './fixtures';
import { FormPage, EntityPage } from '../pages';

// Verifies the markdown editor's inline `@`-triggered entity-reference
// completion menu. Complements the modal picker by covering the
// keyboard-driven inline path.
//
// This replaces the backtick trigger the CodeMirror editor used. The backtick
// flow needed two steps — pick a type prefix, then pick an entity — because a
// backtick says nothing about what the user wants. `@` searches every type at
// once, so the type step is gone.
test.describe('Markdown editor @ mention autocomplete', () => {
  let targetId: string | null = null;
  let originId: string | null = null;
  const targetTitleToken = 'mentionz' + Math.random().toString(36).slice(2, 8);
  const targetTitle = `${targetTitleToken} Target Feature`;

  test.beforeEach(async ({ api }) => {
    const target = await api.createEntity('features', {
      properties: {
        title: targetTitle,
        description: 'Selected via the inline @ completion menu',
        status: 'draft',
        priority: 'medium',
      },
    });
    targetId = target.id;
    // The menu hits /_search, backed by a Bleve index that commits
    // asynchronously after creates.
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

  test('typing @ opens the menu with a prompt to keep typing', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor('see @');
    await form.waitForMentionMenu();
    // A bare `@` has no query yet, so the menu prompts rather than
    // searching every entity in the project.
    await expect(form.mentionMenuNote).toContainText(/type to search/i);
  });

  test('typing a query lists the matching entity', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor(`see @${targetTitleToken}`);
    await form.waitForMentionMenu();
    await expect(form.mentionMenuOptions.first()).toBeVisible({ timeout: 5_000 });
    await expect(form.mentionMenuOptions.first()).toContainText(targetId!);
  });

  test('Enter inserts the reference and closes the menu', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor(`see @${targetTitleToken}`);
    await form.waitForMentionMenu();
    await expect(form.mentionMenuOptions.first()).toBeVisible({ timeout: 5_000 });
    await appPage.keyboard.press('Enter');

    await expect(form.mentionMenu).not.toBeVisible();
    // The trigger and the query are both consumed: only the reference is
    // left, serialized as the code span the store holds.
    const body = await form.getMarkdownBody();
    expect(body).toContain(`\`${targetId}\``);
    expect(body).not.toContain('@' + targetTitleToken);
  });

  test('the inserted reference renders as a titled link inside the editor', async ({
    appPage,
  }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor(`see @${targetTitleToken}`);
    await form.waitForMentionMenu();
    await expect(form.mentionMenuOptions.first()).toBeVisible({ timeout: 5_000 });
    await appPage.keyboard.press('Enter');

    // The title the menu just showed is carried onto the node, so the
    // reference reads correctly straight away rather than waiting for a
    // mentions refresh that would not include a just-inserted ID.
    const ref = form.editorEntityRefs.first();
    await expect(ref).toBeVisible();
    await expect(ref).toHaveText(targetTitle);
    await expect(ref).toHaveAttribute('data-entity-ref', targetId!);
  });

  test('Escape closes the menu without inserting', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor(`see @${targetTitleToken}`);
    await form.waitForMentionMenu();
    await appPage.keyboard.press('Escape');
    await expect(form.mentionMenu).not.toBeVisible();
    const body = await form.getMarkdownBody();
    expect(body).not.toContain(`\`${targetId}\``);
  });

  test('a space after the trigger closes the menu', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor('see @ ');
    // Ordinary prose containing an `@` must not leave a menu hanging open.
    await expect(form.mentionMenu).not.toBeVisible({ timeout: 2_000 });
  });

  test('an email address does not trigger the menu', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor('mail someone@example');
    await expect(form.mentionMenu).not.toBeVisible({ timeout: 2_000 });
  });

  test('round-trip: the saved body renders the rewritten link on the detail page', async ({
    appPage,
    api,
  }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.fillField('title', 'Mention Origin Feature');
    await form.selectField('priority', 'low');
    await form.clearEditorBuffer();
    await form.typeIntoEditor(`see @${targetTitleToken}`);
    await form.waitForMentionMenu();
    await expect(form.mentionMenuOptions.first()).toBeVisible({ timeout: 5_000 });
    await appPage.keyboard.press('Enter');

    const created = await form.submitAndExpectCreate('features');
    originId = created.id;

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', originId!);
    const link = entity.contentEntityRefLink('feature', targetId!);
    await expect(link).toBeVisible();
    await expect(link).toHaveText(targetTitle);

    // The stored markdown is a plain code span, not a link: the title is a
    // per-principal view concern and must never reach the file.
    const persisted = await api.getContent('features', originId!);
    expect(persisted).toContain(`\`${targetId}\``);
    expect(persisted).not.toContain(targetTitle);
  });
});
