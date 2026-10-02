import { test, expect } from './fixtures';
import { EntityPage } from '../pages';
import { SEED } from './fixtures';

// TKT-HOIX1: view section fields render as a view-oriented display value by
// default; `render: input` opts a field into inline editing.
//
// This is the integration case PLAN-6RDYUL flagged as uncoverable by unit
// tests: one page rendering BOTH modes at once. The component tests can prove
// each arm in isolation, but only a real page can show that a display section
// and an inline-edit section coexist without one stealing the other's chrome.
//
// The fixture's `task` view (DATA_ENTRY_YAML in fixtures.ts) is deliberately
// mixed:
//   "Task"          display: properties, no render  → display default
//   "Progress"      display: properties, render: input, widget overrides
//   "Narrow fields" display: properties, render: input, span 2 / 4 / full
//   "Implements"    display: list, render: input     → inline edit
test.describe('View section render modes (TKT-HOIX1)', () => {
  test('renders a display section and an inline-edit section on one page', async ({
    appPage,
  }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // The entry properties section omits `render`, so it must NOT mount an
    // inline-edit form. Pre-TKT-HOIX1 it would have: `status`/`assignee` are
    // ordinarily writable, and writability alone used to decide this.
    await entity.expectEntrySectionRendersDisplay();

    // ...while the list section below it, which opts in, does.
    await entity.expectListSectionRowMounted();
  });

  test('a display-default field renders no form input', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // `status` is an enum on the entry section. Rendered as display it is
    // plain text — no <select>, no FieldShell wrapper.
    await entity.expectEntrySectionHasNoFormControls();

    // The value is still shown — display mode hides the control, not the data.
    // (TASK-001 is seeded with status=draft, assignee=Alice.)
    await entity.expectEntrySectionShowsValue('draft');
    await entity.expectEntrySectionShowsValue('Alice');
  });

  // Regression guard for the display-value staleness the render default makes
  // reachable. `mapFieldsToProperties` used to read the server-side string
  // mirror (`section.fields[i].values`), which the SPA never updates after a
  // PATCH — it rewrites `entry.properties` only. Before TKT-HOIX1 the entry
  // display path was reached only when the ACL denied every field, so nothing
  // could edit those values and the mirror could not go stale. With display as
  // the default it is the common path, so the read now prefers
  // `entry.properties` (same fix as `rowDisplayValue` / RR-FC1C).
  //
  // This asserts the weaker, load-bearing half: a display section shows the
  // CURRENT server value on load, not a mirror that drifted.
  test('a display section reflects the current server value', async ({ appPage, api }) => {
    await api.updateEntity('tasks', SEED.tasks.writeUnitTests, { assignee: 'Bob' });

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    await entity.expectEntrySectionShowsValue('Bob');
    await entity.expectEntrySectionLacksValue('Alice');
  });

  test('an opted-in field is editable', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // The list row's SectionEditForm carries real controls, proving
    // `render: input` reaches the edit arm rather than a disabled widget.
    await entity.expectListSectionRowControlEnabled();
  });

  // BUG-S67G88: a detail field put its fixed-width label beside its value
  // whatever width it was given, so in a span-2 cell the value got none and
  // in a span-4 cell a badge overflowed into the next field. Measured at two
  // widths: at 1100px the span-2 cell is already narrower than that label,
  // and at 780px the span-4 cell is too.
  for (const width of [1100, 780]) {
    test(`labels and values stay inside narrow span cells at ${width}px`, async ({ appPage }) => {
      await appPage.setViewportSize({ width, height: 800 });
      const entity = new EntityPage(appPage);
      await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

      // Guard against a vacuous pass: the case only exists while the cell is
      // narrower than the 120px label column.
      expect(await entity.sectionFieldCellWidth('Narrow fields', 'status')).toBeLessThan(120);
      await entity.expectSectionFieldsFitTheirCells('Narrow fields');
    });
  }

  test('an inline editor keeps its minimum width yet stays inside a narrow cell', async ({
    appPage,
  }) => {
    await appPage.setViewportSize({ width: 780, height: 800 });
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // Full width: the editor keeps the 240px floor it has always had.
    const wide = await entity.measureSectionFieldEditor('Narrow fields', 'note');
    expect(wide.editorWidth).toBeGreaterThanOrEqual(240);

    // A span-4 cell narrower than that floor: the editor fits the cell instead.
    const narrow = await entity.measureSectionFieldEditor('Narrow fields', 'assignee');
    expect(narrow.editorRight).toBeLessThanOrEqual(narrow.cellRight + 0.5);
  });
});
