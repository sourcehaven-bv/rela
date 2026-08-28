import { test, expect } from './fixtures';
import { EntityPage } from '../pages';
import { SEED } from './fixtures';

// TKT-3R7RF3: a view section field can override WHICH widget renders it,
// independent of the type-derived default.
//
// The fixture's `task` view (DATA_ENTRY_YAML in fixtures.ts) carries a
// "Progress" section with `render: input` and two overrides:
//   done  (boolean) widget: checkbox   → also its type default
//   title (string)  widget: textarea   → NOT its type default (that is text)
//
// `title` is the load-bearing case. A boolean already resolves to a checkbox
// on its own, so a checkbox assertion alone would still pass if the override
// were silently dropped. Only the textarea proves config reached the registry.
test.describe('View section widget override (TKT-3R7RF3)', () => {
  test('a string property renders as a textarea when overridden', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // Type default for `string` is TextWidget's <input>. A <textarea> here can
    // only come from `widget: textarea`.
    await entity.expectSectionFieldIsTextarea('Progress', 'title');
  });

  test('an overridden checkbox is interactive, not a display icon', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // Display-mode CheckboxWidget renders a DISABLED checkbox with no id, so
    // matching an enabled `#section-edit-done` proves the edit arm was taken —
    // the "a checkbox you cannot click is just an icon" case from the ticket.
    await entity.expectSectionCheckboxEnabled('Progress', 'done');
  });

  test('clicking the overridden checkbox persists the change', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // Seeded false; toggling must flip it and survive a reload, which is what
    // makes this a real inline edit rather than a local DOM change.
    const after = await entity.toggleSectionCheckbox('Progress', 'done');
    expect(after).toBe(true);

    await appPage.reload();
    await entity.expectSectionCheckboxEnabled('Progress', 'done');
    await entity.expectSectionCheckboxChecked('Progress', 'done');
  });

  // The widget axis must not disturb the render axis TKT-HOIX1 established.
  // The entry section above "Progress" omits `render:`, so it must still be
  // display-rendered even though a sibling section opts into input.
  test('the display-default section is unaffected by a sibling override', async ({
    appPage,
  }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    await entity.expectEntrySectionRendersDisplay();
    await entity.expectEntrySectionHasNoFormControls();
  });
});
