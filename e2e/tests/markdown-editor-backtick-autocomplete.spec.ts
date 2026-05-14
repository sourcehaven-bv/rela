import { test, expect } from './fixtures';
import { FormPage, EntityPage } from '../pages';

// Verifies the markdown editor's inline `-triggered entity-reference
// autocomplete (TKT-2RCP). Complements the modal picker (TKT-I5NO) by
// covering the keyboard-driven inline path.
test.describe('Markdown editor backtick autocomplete', () => {
  let targetId: string | null = null;
  const targetTitleToken = 'tickle' + Math.random().toString(36).slice(2, 8);
  const targetTitle = `${targetTitleToken} Target Feature`;

  test.beforeEach(async ({ api }) => {
    const target = await api.createEntity('features', {
      properties: {
        title: targetTitle,
        description: 'Selected via the inline backtick autocomplete',
        status: 'draft',
        priority: 'medium',
      },
    });
    targetId = target.id;
    await api.waitForIndexed(target.id);
  });

  test.afterEach(async ({ api }) => {
    if (targetId) {
      await api.deleteEntity('features', targetId).catch(() => {});
      targetId = null;
    }
  });

  test('typing a backtick in prose opens the popup after the open delay', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.useFastAutocompleteDelay();
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor('see `');
    await form.waitForBacktickPopup();
    // The popup's first phase lists prefixes — the hint says "Select an
    // entity type".
    await expect(form.backtickPopupHint).toContainText('entity type');
  });

  test('typing a prefix transitions to phase 2 and pressing Enter inserts the id', async ({
    appPage,
  }) => {
    const form = new FormPage(appPage);
    await form.useFastAutocompleteDelay();
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    // Two-step typing: open the popup with `, wait for phase 1, then
    // type the prefix. Splitting avoids racing the typing against the
    // open-delay timer in a single Playwright pump.
    await form.typeIntoEditor('see `');
    await form.waitForBacktickPopup();
    await expect(form.backtickPopupHint).toContainText('entity type');
    await form.typeIntoEditor('FEAT-');
    // After the prefix is typed the popup transitions to phase 2 (id list).
    // The e2e metamodel records the prefix as `FEAT` (no trailing dash),
    // so the hint reads "Entities matching FEAT".
    await expect(form.backtickPopupHint.first()).toContainText('FEAT', { timeout: 2_000 });
    // Wait for the search-debounced results to land.
    await expect(form.backtickPopupOptions.first()).toBeVisible({ timeout: 3_000 });
    // Narrow by partial id so the target lands as the highlighted row.
    await form.typeIntoEditor(targetId!.replace(/^FEAT-/, ''));
    await expect(form.backtickPopupOptions.first()).toContainText(targetId!);
    await appPage.keyboard.press('Enter');
    // Popup closes; buffer contains `<id>`.
    await expect(form.backtickPopup).not.toBeVisible();
    const body = await form.getMarkdownBody();
    expect(body).toContain(`\`${targetId}\``);
  });

  test('typing a backtick inside a fenced code block does NOT open the popup', async ({
    appPage,
  }) => {
    const form = new FormPage(appPage);
    await form.useFastAutocompleteDelay();
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    // Set up a fenced block: ```\n...\n```. Then type a backtick inside.
    await form.typeIntoEditor('```\nlet x = ');
    await form.typeIntoEditor('`');
    // The popup must NEVER appear. Wait a generous beat past the
    // open-delay window — if the popup is going to appear it'll be
    // within ~700 ms; `not.toBeVisible` polls until the timeout, then
    // passes if still hidden.
    await expect(form.backtickPopup).not.toBeVisible({ timeout: 1_200 });
  });

  test('typing `flag-name` quickly does NOT show the popup', async ({ appPage }) => {
    // The open-delay grace period (~600ms) ensures literal code spans
    // typed at normal speed never flash a popup. typeIntoEditor
    // synthesises a non-id character (the hyphen) within ms of the
    // backtick, so the delay expires onto a disqualifying buffer state.
    const form = new FormPage(appPage);
    await form.useFastAutocompleteDelay();
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor('`flag-name');
    await expect(form.backtickPopup).not.toBeVisible({ timeout: 1_200 });
  });

  test('Escape dismisses the popup without inserting', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.useFastAutocompleteDelay();
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    const before = await form.getMarkdownBody();
    await form.typeIntoEditor('see `');
    await form.waitForBacktickPopup();
    await appPage.keyboard.press('Escape');
    await expect(form.backtickPopup).not.toBeVisible();
    const after = await form.getMarkdownBody();
    // Esc closed the popup — the typed backtick (plus auto-pair) stays.
    expect(after.startsWith(before)).toBe(true);
    expect(after).toContain('see `');
  });

  test('round-trip: inline insert produces a titled link on the detail page', async ({
    appPage,
    api,
  }) => {
    const form = new FormPage(appPage);
    await form.useFastAutocompleteDelay();
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.fillField('title', 'Backtick autocomplete round-trip');
    await form.selectField('priority', 'low');
    await form.clearEditorBuffer();
    await form.typeIntoEditor('`');
    await form.waitForBacktickPopup();
    await form.typeIntoEditor('FEAT-');
    await expect(form.backtickPopupOptions.first()).toBeVisible({ timeout: 3_000 });
    await form.typeIntoEditor(targetId!.replace(/^FEAT-/, ''));
    await expect(form.backtickPopupOptions.first()).toContainText(targetId!);
    await appPage.keyboard.press('Enter');

    const created = await form.submitAndExpectCreate('features');
    try {
      const entity = new EntityPage(appPage);
      await entity.navigateToEntity('feature', created.id);
      const link = entity.contentEntityRefLink('feature', targetId!);
      await expect(link).toBeVisible();
      await expect(link).toHaveText(targetTitle);
    } finally {
      await api.deleteEntity('features', created.id).catch(() => {});
    }
  });
});
