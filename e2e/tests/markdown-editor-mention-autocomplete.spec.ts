import { test, expect } from './fixtures';
import { FormPage, EntityPage } from '../pages';

// Verifies the markdown editor's inline `@`-triggered entity-reference
// completion menu. Complements the modal picker by covering the
// keyboard-driven inline path.
//
// This replaces the backtick trigger the CodeMirror editor used, which REQUIRED
// two steps: pick a type prefix, then pick an entity. `@` searches every type at
// once, so that step is optional rather than gone — a type picker is offered
// while the query is short, and choosing one scopes the search. Picking a type
// narrows; it never inserts.
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

  test('typing @ offers the entity types to scope by', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor('see @');
    await form.waitForMentionMenu();
    // A bare `@` still fires no search, but the slot is no longer a dead
    // "type to search" note: listing the types is how the picker is found.
    await expect(form.mentionMenuTypeOptions.first()).toBeVisible();
    await expect(form.mentionMenuEntityOptions).toHaveCount(0);
  });

  test('one letter already narrows the type list', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();

    // A bare `@` offers every type — that is the discovery path. `bug` is one of
    // them and does not match `f`, so it is the witness that narrowing happened.
    await form.typeIntoEditor('see @');
    await form.waitForMentionMenu();
    await expect(form.mentionMenuTypeOptions.filter({ hasText: 'bug' })).toHaveCount(1);

    // One letter narrows. Deliberately NOT gated on MIN_SEARCH_LEN: filtering
    // type names is local, so it reacts from the first keystroke while the
    // entity search still waits for the second character.
    //
    // Assert on CONTENT, not on a row count read once: the count is captured
    // before Vue re-renders and reads the pre-keystroke list, which is exactly
    // how the first version of this test failed against working code. `expect`
    // auto-retries, a bare `count()` does not.
    await form.typeIntoEditor('f');
    await form.expectEditorText('see @f');
    await expect(form.mentionMenuTypeOptions.filter({ hasText: 'feature' })).toHaveCount(1);
    await expect(form.mentionMenuTypeOptions.filter({ hasText: 'bug' })).toHaveCount(0);
    // Still capped at MAX_TYPE_SUGGESTIONS, and never empty for a real prefix.
    await expect(form.mentionMenuTypeOptions).not.toHaveCount(0);
  });

  test('the arrow keys move the highlight, one row at a time', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    // A bare `@` lists every type, so there are several rows to traverse.
    await form.typeIntoEditor('see @');
    await form.waitForMentionMenu();
    await expect(form.mentionMenuOptions.first()).toBeVisible();
    expect(await form.mentionMenuOptions.count()).toBeGreaterThan(2);

    const highlighted = form.mentionMenuHighlighted;
    // Exactly one row is highlighted at every point, and it actually moves.
    // The highlight is stored as an identity and resolved against the live
    // rows, so this also covers that the resolution reaches the template.
    await expect(highlighted).toHaveCount(1);
    const first = await highlighted.innerText();

    await appPage.keyboard.press('ArrowDown');
    await expect(highlighted).toHaveCount(1);
    const second = await highlighted.innerText();
    expect(second).not.toBe(first);

    await appPage.keyboard.press('ArrowUp');
    await expect(highlighted).toHaveCount(1);
    expect(await highlighted.innerText()).toBe(first);
  });

  // Focus stays in the editor, so the rows are never focused and the ONLY way a
  // screen reader learns which row is active is `aria-activedescendant` naming
  // it. A dangling idref reads as nothing at all, so this asserts the reference
  // actually resolves to the highlighted element rather than merely being set.
  test('the listbox names the active row for assistive tech', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor('see @');
    await form.waitForMentionMenu();
    await expect(form.mentionMenuOptions.first()).toBeVisible();

    const probe = () =>
      appPage.evaluate(() => {
        const box = document.querySelector('.mention-menu[role="listbox"]');
        const id = box?.getAttribute('aria-activedescendant') ?? null;
        // Resolve WITHIN this listbox, not via document.getElementById. useId()
        // ids are document-scoped, so a page holding two editors would let the
        // wrong instance's row satisfy the lookup and the assertion would pass
        // for the wrong reason.
        const target = id ? box?.querySelector(`#${CSS.escape(id)}`) : null;
        return {
          // One listbox for one selection: the highlight is a single sequence
          // across both sections, so nested listboxes would misreport it.
          nested: document.querySelectorAll('.mention-menu [role="listbox"]').length,
          resolves: target !== null,
          onHighlighted: target?.classList.contains('is-highlighted') ?? false,
          id,
        };
      });

    const before = await probe();
    expect(before.nested).toBe(0);
    expect(before.resolves).toBe(true);
    expect(before.onHighlighted).toBe(true);

    await appPage.keyboard.press('ArrowDown');
    const after = await probe();
    expect(after.resolves).toBe(true);
    expect(after.onHighlighted).toBe(true);
    // It tracks the highlight rather than being set once and left behind.
    expect(after.id).not.toBe(before.id);
  });

  test('typing a query lists the matching entity', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    await form.typeIntoEditor(`see @${targetTitleToken}`);
    await form.waitForMentionMenu();
    await expect(form.mentionMenuEntityOptions.first()).toBeVisible({ timeout: 5_000 });
    await expect(form.mentionMenuEntityOptions.first()).toContainText(targetId!);
    // The token is longer than the type-section threshold, so the picker is
    // out of the way and Enter is unambiguously "insert this entity".
    await expect(form.mentionMenuTypeOptions).toHaveCount(0);
  });

  test('picking a type scopes the search and Enter then inserts an entity', async ({
    appPage,
  }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();

    // A short query keeps the type section visible; `feature` is the type the
    // fixture entity belongs to.
    await form.typeIntoEditor('see @featur');
    // Gate on the query actually landing: a dropped keystroke would otherwise
    // fail below as a missing type row, which reads like a product bug.
    await form.expectEditorText('see @featur');
    await form.waitForMentionMenu();
    const featureType = form.mentionMenuTypeOptions.filter({ hasText: 'feature' }).first();
    await expect(featureType).toBeVisible({ timeout: 5_000 });
    await featureType.click();

    // The scope is now visible as a chip, and the type rows are gone.
    await expect(form.mentionMenuScopeChip).toHaveText('feature');
    await expect(form.mentionMenuTypeOptions).toHaveCount(0);

    // Picking a type scopes the search; it does NOT rewrite the document, so
    // the `featur` that surfaced the picker is still in the query and has to be
    // deleted before searching for the entity. The chip survives those
    // Backspaces because the query is non-empty throughout.
    for (let i = 0; i < 'featur'.length; i++) await appPage.keyboard.press('Backspace');
    await expect(form.mentionMenuScopeChip).toHaveText('feature');

    await form.typeIntoEditor(targetTitleToken);
    await expect(form.mentionMenuEntityOptions.first()).toBeVisible({ timeout: 5_000 });
    await expect(form.mentionMenuEntityOptions.first()).toContainText(targetId!);

    await appPage.keyboard.press('Enter');
    await expect(form.mentionMenu).not.toBeVisible();
    const body = await form.getMarkdownBody();
    expect(body).toContain(`\`${targetId}\``);
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

  // This one has to be an e2e test, and it has to press Backspace back-to-back
  // with no waits between keystrokes. The bug it guards against was a stale
  // read of the menu's own `query` mirror, which the slash provider writes on
  // ProseMirror's update cycle — one tick AFTER the capture-phase key handler.
  // Any pause between keystrokes lets the mirror catch up and the bug vanishes,
  // which is why an earlier single-worker run with waits passed while the real
  // suite failed. A unit test cannot see this at all: there is no ProseMirror
  // update cycle in jsdom.
  test('Backspace at the start of the query clears the type scope', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();
    const query = 'featur';
    await form.typeIntoEditor(`see @${query}`);
    await form.expectEditorText(`see @${query}`);
    await form.waitForMentionMenu();
    const featureType = form.mentionMenuTypeOptions.filter({ hasText: 'feature' }).first();
    await expect(featureType).toBeVisible({ timeout: 5_000 });
    await featureType.click();
    await expect(form.mentionMenuScopeChip).toHaveText('feature');

    // Backspace with characters left deletes text, not the scope: the chip
    // survives every keystroke that still has query to consume. Stop one short
    // of empty so the chip is unambiguously still there.
    for (let i = 0; i < query.length - 1; i++) await appPage.keyboard.press('Backspace');
    await form.expectEditorText('see @f');
    await expect(form.mentionMenuScopeChip).toHaveText('feature');

    // Emptying the query does not clear the chip either.
    await appPage.keyboard.press('Backspace');
    await expect(form.mentionMenuScopeChip).toHaveText('feature');

    // Only the NEXT Backspace, on an already-empty query, clears it — and the
    // `@` is preserved (the handler swallows that keystroke), so the menu stays
    // open and the type rows return with the search unscoped.
    await appPage.keyboard.press('Backspace');
    await expect(form.mentionMenuScopeChip).toHaveCount(0);
    await expect(form.mentionMenuTypeOptions.first()).toBeVisible();
    await form.expectEditorText('see @');
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
