import { test, expect, SEED } from './fixtures';
import { EntityPage, FormPage } from '../pages';

/**
 * Body content with GFM checkboxes renders as interactive checkboxes in the
 * entity detail view. Clicking toggles the markdown source on the server.
 *
 * SEED.features.checkboxBody (FEAT-004) is the dedicated fixture: its body has
 * one unchecked and one checked item. Specs read or flip the rendered state;
 * there is no per-test API setup.
 */

test.describe('Checkbox toggling', () => {
  test('entity detail shows checkbox stats for content with checkboxes', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', SEED.features.checkboxBody);

    // Stats widget renders when the body contains checkboxes; format is "n/m".
    if (await entity.hasCheckboxStats()) {
      expect(await entity.getCheckboxStatsText()).toMatch(/\d+\/\d+/);
    }

    expect(await entity.contentCheckboxCount()).toBeGreaterThanOrEqual(2);
  });

  test('clicking a checkbox persists the toggle on the server', async ({ appPage, api }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', SEED.features.checkboxBody);

    expect(await entity.contentCheckboxCount()).toBeGreaterThanOrEqual(2);
    await entity.clickContentCheckbox(0);

    // The first line starts unchecked; after a click the server-side content
    // should report it checked. Re-read via the API to avoid racing SSE and
    // frontend re-render timing.
    await expect
      .poll(
        async () => {
          const content = await api.getContent('features', SEED.features.checkboxBody);
          const firstLine = content.split('\n')[0] ?? '';
          return /- \[x\]/i.test(firstLine);
        },
        { timeout: 5000 },
      )
      .toBe(true);

    // ...and the SPA's rendered state must visibly reflect the new value.
    // Server-state alone passing would be the exact failure shape of the
    // original bug (API works, UI doesn't); assert end-to-end.
    await expect.poll(() => entity.contentCheckboxIsChecked(0), { timeout: 2000 }).toBe(true);
  });

  test('toggling a checkbox does not flicker the entity detail tree', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', SEED.features.checkboxBody);
    expect(await entity.contentCheckboxCount()).toBeGreaterThanOrEqual(2);

    // The pre-TKT-R7Q9 toggle path called loadView() after every click,
    // which flipped loading.value=true → v-if="loading" branch → entire
    // entity-detail tree torn down and rebuilt (visible flicker). The
    // PATCH-based reactive flow mutates only viewData.entry.content +
    // the entry-content section, so the EntityDetail's .loading-state
    // spinner must never appear during a toggle.
    //
    // Install a MutationObserver inside the page BEFORE the click so a
    // sub-frame loading flip can't slip between polls. Scoped to
    // `.entity-detail > .loading-state` (direct child) so we don't catch
    // unrelated DocumentsPanel / SidePanel / HelpModal spinners that share
    // the same CSS class.
    await appPage.evaluate(() => {
      const w = window as unknown as { __entityDetailLoadingSeen?: boolean };
      w.__entityDetailLoadingSeen = false;
      const observer = new MutationObserver(() => {
        if (document.querySelector('.entity-detail > .loading-state')) {
          w.__entityDetailLoadingSeen = true;
        }
      });
      observer.observe(document.body, { childList: true, subtree: true });
      // Also capture state at install time (in case it's already showing).
      if (document.querySelector('.entity-detail > .loading-state')) {
        w.__entityDetailLoadingSeen = true;
      }
    });

    await entity.clickContentCheckbox(0);
    await expect
      .poll(() => entity.contentCheckboxIsChecked(0), { timeout: 2000 })
      .toBe(true);

    const loadingSeen = await appPage.evaluate(
      () => (window as unknown as { __entityDetailLoadingSeen?: boolean }).__entityDetailLoadingSeen ?? false,
    );
    expect(loadingSeen, 'entity-detail loading spinner appeared during checkbox toggle').toBe(false);
  });
});

/**
 * The same body, opened in the Milkdown editor (BUG-KHQXHH).
 *
 * The unit suite (`frontend/src/components/forms/milkdown/taskList.test.ts`)
 * covers the DOM shape and the toggle behaviour. These two cases exist because
 * jsdom cannot decide them: the bullet suppression relies on `:has()`, and the
 * inline layout needs a real layout engine. Both are what actually makes a
 * checklist readable, and both would regress silently in the unit suite.
 */
test.describe('Checkboxes in the markdown editor', () => {
  test('a task item renders one checkbox and no bullet', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToEditForm('feature', SEED.features.checkboxBody);

    await expect(form.editorTaskItems.first()).toBeVisible();
    const count = await form.editorTaskItems.count();
    expect(count).toBeGreaterThanOrEqual(2);

    // One real checkbox per item — the element the shared `.md-body` rules
    // select on, and the one Milkdown's own schema never emits.
    await expect(form.editorTaskCheckboxes).toHaveCount(count);

    // `list-style: none` comes from the `:has()` rule. If the node view stops
    // putting the input first the marker returns, and the row shows a bullet
    // AND a box.
    for (let i = 0; i < count; i++) {
      expect(await form.editorTaskListStyle(i), `item ${i} kept its bullet`).toBe('none');
    }

    // The fixture has one unchecked and one checked item; the editor must show
    // that difference rather than rendering them identically, which is the
    // user-visible symptom of the bug.
    const states = await form.editorTaskCheckedStates();
    expect(new Set(states).size, 'checked and unchecked items look the same').toBe(2);
  });

  test('the label sits beside the checkbox, not below it', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToEditForm('feature', SEED.features.checkboxBody);
    await expect(form.editorTaskItems.first()).toBeVisible();

    // The node view's content wrapper has no counterpart in the rendered HTML,
    // so it is styled only by milkdownEditor.css. Left block-level it pushes
    // the text onto the next line, which reads as an empty task item above a
    // stray paragraph.
    const { checkbox, content } = await form.editorTaskRowBoxes(0);
    expect(checkbox, 'no checkbox').not.toBeNull();
    expect(content, 'no content wrapper').not.toBeNull();
    expect(content!.x).toBeGreaterThan(checkbox!.x);
    // Same visual row: vertical centres within one line of each other.
    expect(
      Math.abs(content!.y + content!.height / 2 - (checkbox!.y + checkbox!.height / 2)),
    ).toBeLessThan(12);

    // The wrapper's inner paragraph must not keep the shared sheet's 14px
    // bottom margin, or every task row gains a blank line under it. The
    // position assertion above does NOT catch this — it was verified to still
    // pass with the margin present.
    expect(await form.editorTaskParagraphMarginBottom(0)).toBe('0px');
  });
});

/**
 * Toggling in the editor, in a real browser (BUG-KHQXHH).
 *
 * The keyboard path is the one that was broken and is the one jsdom models
 * least faithfully: canceling a checkbox's click makes the HTML spec restore
 * the pre-click checkedness after the handler returns, so both the box and the
 * document have to be verified together in a real engine.
 */
test.describe('Toggling task lists in the markdown editor', () => {
  test('a mouse click updates both the box and the document', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToEditForm('feature', SEED.features.checkboxBody);
    await expect(form.editorTaskItems.first()).toBeVisible();
    expect(await form.editorTaskCheckedStates()).toEqual([false, true]);

    await form.clickEditorTaskCheckbox(0);
    await expect.poll(() => form.editorTaskCheckedStates()).toEqual([true, true]);
    expect(await form.getMarkdownBody()).toContain('- [x] First');
  });

  test('Space on a focused checkbox updates both the box and the document', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToEditForm('feature', SEED.features.checkboxBody);
    await expect(form.editorTaskItems.first()).toBeVisible();

    await form.pressSpaceOnEditorTaskCheckbox(0);
    // Both halves matter. Before the fix the box ticked and the document did
    // not, so the edit was lost on the next redraw with nothing to warn the
    // user — asserting only the box would still have passed.
    await expect.poll(() => form.editorTaskCheckedStates()).toEqual([true, true]);
    expect(await form.getMarkdownBody()).toContain('- [x] First');
  });
});
