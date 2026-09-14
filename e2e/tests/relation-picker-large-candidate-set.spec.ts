import { test, expect, SEED } from './fixtures';
import { FormPage } from '../pages';

/**
 * BUG-HOB9BR / GitHub #1595 — a legacy (non-cards) RelationPicker silently
 * refused to save when an already-linked target sat outside the first page of
 * candidates.
 *
 * The picker resolves each selected ID to its entity TYPE from its own
 * `candidates` array, and that array WAS one `fetchList(..., per_page: 100)`
 * call — a single page, not the whole collection. The type map it emitted on
 * `update:types` therefore covered only the first 100 entities of the target
 * type. `reshapeLegacyToModern` needs a type for EVERY id it is handed and
 * returns null when one is missing, which made `buildAutoSaveRelationsBody`
 * abort the form's entire relations autosave with "Some related entities have
 * unknown types" and send no PATCH at all.
 *
 * Fixed by paging the candidate load (`fetchAllList`).
 *
 * The user-visible symptom is the worst kind: the chip appears, nothing is
 * written, and a reload shows the edit gone. Same failure class as BUG-5OAQUG
 * on the kanban board, which `listAllEntities` fixed for that consumer only.
 *
 * The seed below is the minimal shape that reproduces it: more than 100
 * features so the collection pages, with the task's existing link pointing at
 * one past the page boundary. The API orders `/api/v1/features` by ID
 * ascending, so a feature seeded after the 100th is reliably off page 1.
 */

// One over the server's per-page cap of 100, so exactly one feature falls
// outside the window the picker loads.
const EXTRA_FEATURES = 100;

test.describe('Relation picker with a paged candidate set', () => {
  test('adds a second value when the existing link is outside the first candidate page (BUG-HOB9BR)', async ({
    appPage,
    api,
  }) => {
    // Seed past the page boundary. The four seeded features (FEAT-001..004)
    // already exist, so this pushes the total to 104 and the last-created
    // feature lands on page 2.
    let offPageFeatureId = '';
    for (let i = 0; i < EXTRA_FEATURES; i++) {
      const created = await api.createEntity('features', {
        properties: { title: `Bulk feature ${String(i).padStart(3, '0')}`, status: 'draft' },
      });
      offPageFeatureId = created.id;
    }

    // Confirm the premise rather than trusting the arithmetic: the picker
    // loads exactly one page, so the test is only meaningful if this feature
    // is genuinely absent from it. If the cap or the ordering ever changes,
    // fail here with a clear reason instead of passing vacuously.
    const page1 = await api.listEntities('features', 'per_page=100&page=1');
    expect(
      page1.data.map((e) => e.id),
      'the off-page feature must not be on page 1, or the test proves nothing',
    ).not.toContain(offPageFeatureId);

    // TASK-001 already implements FEAT-001 (seeded). Point it at the off-page
    // feature instead: that is the link whose type the picker cannot resolve.
    await api.createRelation('tasks', SEED.tasks.writeUnitTests, 'implements', offPageFeatureId);

    const form = new FormPage(appPage);
    await form.navigateToEditForm('task', SEED.tasks.writeUnitTests);

    const picker = form.relationPickerByLabel('Implements Feature');
    await expect(picker).toBeVisible();

    // Add a second target that IS on page 1, mirroring the report: the newly
    // picked entity resolves fine, the pre-existing one does not.
    await form.pickInRelationPicker(picker, 'Dashboard Analytics', 'Dashboard Analytics');
    await expect(form.pickerTileByText(picker, 'Dashboard Analytics')).toBeVisible();

    await form.saveAndWaitForNavigation();

    // The assertion that matters is the server's state, not the toast: the bug
    // sent no PATCH at all, so both edges must be readable afterwards.
    const links = await api.listRelations('tasks', SEED.tasks.writeUnitTests, 'implements');
    const linkedIds = links.map((r) => r.id);
    expect(linkedIds).toContain(SEED.features.dashboardAnalytics);
    expect(
      linkedIds,
      'the pre-existing off-page link must survive the save, not be dropped',
    ).toContain(offPageFeatureId);
  });
});
