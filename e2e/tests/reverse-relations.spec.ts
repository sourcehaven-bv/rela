import { test, expect, SEED } from './fixtures';
import { RelationCardsPage } from '../pages';

/**
 * Reverse (incoming) relation tests.
 *
 * The inline fixture seeds:
 *   - `FEAT-001 --blocks--> FEAT-003` (cards-widget reverse on FEAT-003)
 *   - `TASK-001 --implements--> FEAT-001` (non-cards picker reverse on FEAT-001)
 *
 * The feature form declares two `blocks` widgets (outgoing + incoming, both
 * `widget: cards`) plus an `implements` widget with `direction: incoming` and
 * no `widget:` (defaults to RelationPicker since the relation has no
 * properties).
 *
 * Regression target: BOTH widget paths must render incoming edges. The
 * non-cards path used to silently ignore `direction: incoming` because
 * RelationPicker had no awareness of the field's direction and
 * `entity.relations` only carries outgoing edges.
 */

test.describe('Reverse relations', () => {
  test('backend returns incoming blocks edge for FEAT-003', async ({ api }) => {
    const resp = await api.rawRequest(
      'GET',
      `features/${SEED.features.exportData}/relations/blocks?direction=incoming`,
    );
    expect(resp.ok()).toBeTruthy();
    const edges = (await resp.json()) as Array<{ id: string; meta?: Record<string, unknown> }>;
    expect(edges.map((e) => e.id)).toContain(SEED.features.authentication);
    const edge = edges.find((e) => e.id === SEED.features.authentication);
    expect(edge?.meta?.reason).toBe('test block');
  });

  test('backend grouped relations response labels incoming under inverse name', async ({ api }) => {
    // GET /api/v1/{plural}/{id}/relations groups edges by type; incoming
    // edges surface under the relation's configured inverse (`blockedBy`).
    const resp = await api.rawRequest('GET', `features/${SEED.features.exportData}/relations`);
    expect(resp.ok()).toBeTruthy();
    const grouped = (await resp.json()) as Record<string, Array<{ id: string; direction: string }>>;
    const blockedBy = grouped['blockedBy'] ?? [];
    expect(blockedBy.map((e) => e.id)).toContain(SEED.features.authentication);
    expect(blockedBy.every((e) => e.direction === 'incoming')).toBeTruthy();
  });

  test('cards widget renders incoming blocks card for FEAT-003 (from FEAT-001)', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.exportData);

    // Both widgets share the label "blocks" (no custom label in fixture).
    // FEAT-003 has zero outgoing blocks and one incoming block from FEAT-001,
    // so the FEAT-001 card must appear in exactly one widget — the incoming
    // one. Assert via a page-wide card locator rather than guessing which of
    // the two "blocks"-labeled widgets is which.
    const incomingCard = rc.cardByTargetId(SEED.features.authentication);
    await expect(incomingCard).toBeVisible();
  });

  test('cards widget shows relation meta on the reverse card', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.exportData);

    const card = rc.cardByTargetId(SEED.features.authentication);
    await expect(card).toBeVisible();
    // The seeded reason is "test block"; it must round-trip through the
    // incoming-direction API path to the card's inline input.
    expect(await rc.getTextInputValue(card)).toBe('test block');
  });

  test('cards widget edit + save persists via the reversed endpoint', async ({ appPage, api }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.exportData);

    const card = rc.cardByTargetId(SEED.features.authentication);
    await expect(card).toBeVisible();

    const updated = 'reverse-edit persists';
    await rc.editTextInput(card, updated);
    await rc.saveAndWaitForNavigation();

    // The relation is stored FROM FEAT-001 TO FEAT-003; its canonical read
    // path is the outgoing blocks of FEAT-001. If reverse-direction save
    // works, the reason must be updated on that underlying edge.
    const fromSource = await api.listRelations(
      'features',
      SEED.features.authentication,
      'blocks',
    );
    const edge = fromSource.find((r) => r.id === SEED.features.exportData);
    expect(edge?.meta?.reason).toBe(updated);
  });

  test('non-cards picker lists linked source entities with direction: incoming', async ({ appPage, api }) => {
    // The 'implements' relation goes task -> feature. On the FEAT-001 form
    // we configure a non-cards widget with direction: incoming labelled
    // "Implemented by". TASK-001 implements FEAT-001 in the seed, so the
    // widget must render TASK-001 as a linked entity.
    //
    // Backend contract check first.
    const resp = await api.rawRequest(
      'GET',
      `features/${SEED.features.authentication}/relations/implements?direction=incoming`,
    );
    expect(resp.ok()).toBeTruthy();
    const edges = (await resp.json()) as Array<{ id: string }>;
    expect(edges.map((e) => e.id)).toContain(SEED.tasks.writeUnitTests);

    // UI must show the linked task as a selected-entity tile inside the
    // "Implemented by" widget. The picker renders entity type + title in the
    // tile (no ID), so assert on the seeded TASK-001 title.
    await appPage.goto(`${new URL(appPage.url()).origin}/form/feature/${SEED.features.authentication}`);
    const picker = appPage.locator('.form-field', { hasText: 'Implemented by' }).first();
    await expect(picker).toBeVisible();
    const tile = picker.locator('.selected-entity', { hasText: 'Write unit tests' });
    await expect(tile).toBeVisible();
  });
});
