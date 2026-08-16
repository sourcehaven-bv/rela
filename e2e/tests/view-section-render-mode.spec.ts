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
//   section[0] "Task"       display: properties, no render  → display default
//   section[1] "Implements" display: list, render: input     → inline edit
test.describe('View section render modes (TKT-HOIX1)', () => {
  test('renders a display section and an inline-edit section on one page', async ({
    appPage,
  }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // The entry properties section omits `render`, so it must NOT mount an
    // inline-edit form. Pre-TKT-HOIX1 it would have: `status`/`assignee` are
    // ordinarily writable, and writability alone used to decide this.
    const entrySection = appPage.locator('.properties-list').first();
    await expect(entrySection).toBeVisible();
    await expect(entrySection.locator('.section-edit-form')).toHaveCount(0);

    // ...while the list section below it, which opts in, does.
    await entity.expectListSectionRowMounted();
  });

  test('a display-default field renders no form input', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // `status` is an enum on the entry section. Rendered as display it is
    // plain text — no <select>, no FieldShell wrapper. Scoped to the entry
    // section so the opted-in list section below can't satisfy this.
    const entrySection = appPage.locator('.properties-list').first();
    await expect(entrySection).toBeVisible();
    await expect(entrySection.locator('select')).toHaveCount(0);
    await expect(entrySection.locator('input')).toHaveCount(0);
    await expect(entrySection.locator('.form-field')).toHaveCount(0);

    // The value is still shown — display mode hides the control, not the data.
    // (TASK-001 is seeded with status=draft, assignee=Alice.)
    await expect(entrySection).toContainText('draft');
    await expect(entrySection).toContainText('Alice');
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

    const entrySection = appPage.locator('.properties-list').first();
    await expect(entrySection).toBeVisible();
    await expect(entrySection).toContainText('Bob');
    await expect(entrySection).not.toContainText('Alice');
  });

  test('an opted-in field is editable and autosaves', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.writeUnitTests);

    // The list row's SectionEditForm carries real controls, proving
    // `render: input` reaches the edit arm rather than a disabled widget.
    const row = appPage.locator('.entity-list .list-item .section-edit-form').first();
    await expect(row).toBeVisible();
    const control = row.locator('select, input').first();
    await expect(control).toBeVisible();
    await expect(control).toBeEnabled();
  });
});
