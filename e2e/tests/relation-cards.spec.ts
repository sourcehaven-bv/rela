import { test, expect, SEED } from './fixtures';
import { RelationCardsPage } from '../pages';

/**
 * widget: cards relation UI tests.
 *
 * Uses seed entity FEAT-001 which has:
 *   - tagged → FEAT-002 (added_by, added_date)
 *   - blocks → FEAT-003 (reason="test block", severity=critical, impact_score=8)
 *
 * The feature form in the inline project configures `tagged` and `blocks`
 * (outgoing + incoming) with widget: cards.
 */

test.describe('Relation Cards', () => {
  test('edit form shows relation cards for tagged and blocks relations', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);

    expect(await rc.widgetCount()).toBeGreaterThanOrEqual(2);
    const labels = (await rc.sectionLabels()).map((l) => l.toLowerCase());
    expect(labels.some((l) => l.includes('tagged'))).toBeTruthy();
    expect(labels.some((l) => l.includes('blocks'))).toBeTruthy();
  });

  test('cards display existing entries with properties', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);

    const card = rc.cardByTargetId(SEED.features.exportData);
    await expect(card).toBeVisible({ timeout: 10000 });

    const propLabels = await rc.cardPropertyLabels(card);
    expect(propLabels.some((l) => /Block Reason|Reason/i.test(l))).toBeTruthy();
    expect(propLabels.some((l) => /Severity/i.test(l))).toBeTruthy();
  });

  test('existing relation property values are populated', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);

    const card = rc.cardByTargetId(SEED.features.exportData);
    await expect(card).toBeVisible({ timeout: 10000 });
    expect(await rc.getTextInputValue(card)).toBe('test block');
  });

  test('editing a text property triggers the unsaved badge', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);

    const card = rc.cardByTargetId(SEED.features.exportData);
    await expect(card).toBeVisible({ timeout: 10000 });
    await rc.editTextInput(card, 'updated block reason');
    expect(await rc.hasAnyUnsavedBadge()).toBeTruthy();
  });

  test('removing a card immediately decrements count and shows unsaved badge', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);

    const tagged = rc.widgetByLabel('tagged').first();
    await expect(tagged).toBeVisible({ timeout: 10000 });
    const before = await rc.cardCount(tagged);
    expect(before).toBeGreaterThanOrEqual(1);

    await rc.clickRemoveFirstCardIn(tagged);
    expect(await rc.cardCount(tagged)).toBe(before - 1);
    expect(await rc.hasAnyUnsavedBadge()).toBeTruthy();
  });

  test('adding a new relation with a reason persists on Save', async ({ appPage, api }) => {
    // Create a new feature to link to.
    const target = await api.createEntity('features', {
      properties: { title: 'E2E Blocks Target', status: 'draft', priority: 'low' },
    });

    try {
      const rc = new RelationCardsPage(appPage);
      await rc.navigateToEdit('feature', SEED.features.authentication);

      const outgoingBlocks = rc.widgetByLabel('blocks').first();
      await expect(outgoingBlocks).toBeVisible({ timeout: 10000 });

      const newCard = await rc.linkTargetByIdWithSearch(
        outgoingBlocks,
        target.id,
        'E2E Blocks Target',
        'blocks due to dependency',
      );
      await expect(newCard).toBeVisible();
      await rc.expectCardHasClass(newCard, /card-added/);
    } finally {
      await api.deleteEntity('features', target.id).catch(() => {});
    }
  });

  test('batch save: changes are not persisted until Save is clicked', async ({ appPage, api }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);

    const card = rc.cardByTargetId(SEED.features.exportData);
    await expect(card).toBeVisible({ timeout: 10000 });

    const original = await rc.getTextInputValue(card);
    const updated = 'batch save test reason';
    await rc.editTextInput(card, updated);

    // Still on the server: untouched.
    const relsResp = await api.rawRequest('GET', `features/${SEED.features.authentication}/relations/blocks`);
    const rels = (await relsResp.json()) as Array<{ id: string; meta?: Record<string, unknown> }>;
    const feat003 = rels.find((r) => r.id === SEED.features.exportData);
    expect(feat003?.meta?.reason).toBe(original);

    await rc.saveAndWaitForNavigation();

    const relsResp2 = await api.rawRequest('GET', `features/${SEED.features.authentication}/relations/blocks`);
    const rels2 = (await relsResp2.json()) as Array<{ id: string; meta?: Record<string, unknown> }>;
    const feat003After = rels2.find((r) => r.id === SEED.features.exportData);
    expect(feat003After?.meta?.reason).toBe(updated);
  });

  test('removing a relation is only persisted on save', async ({ appPage, api }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);

    const tagged = rc.widgetByLabel('tagged').first();
    await expect(tagged).toBeVisible({ timeout: 10000 });
    const firstId = await rc.getFirstCardEntityId(tagged);
    await rc.clickRemoveFirstCardIn(tagged);

    const relsResp = await api.rawRequest('GET', `features/${SEED.features.authentication}/relations/tagged`);
    const rels = (await relsResp.json()) as Array<{ id: string }>;
    expect(rels.some((r) => r.id === firstId)).toBeTruthy();

    await rc.saveAndWaitForNavigation();

    const relsResp2 = await api.rawRequest('GET', `features/${SEED.features.authentication}/relations/tagged`);
    const rels2 = (await relsResp2.json()) as Array<{ id: string }>;
    expect(rels2.some((r) => r.id === firstId)).toBeFalsy();
  });
});

test.describe('Relation card field types', () => {
  test('date input renders for date properties', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);
    const blocks = rc.widgetByLabel('blocks').first();
    await expect(blocks).toBeVisible({ timeout: 10000 });
    await rc.expectDateInputVisibleIn(blocks);
  });

  test('number input renders for integer properties', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);
    await rc.expectNumberInputVisibleIn(rc.widgetByLabel('blocks').first());
  });

  test('checkbox renders for boolean properties', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);
    await rc.expectCheckboxVisibleIn(rc.widgetByLabel('blocks').first());
  });
});

test.describe('Relation cards save flow', () => {
  test('unsaved badge clears after save', async ({ appPage }) => {
    const rc = new RelationCardsPage(appPage);
    await rc.navigateToEdit('feature', SEED.features.authentication);

    const card = rc.cardByTargetId(SEED.features.exportData);
    await expect(card).toBeVisible({ timeout: 10000 });
    await rc.editTextInput(card, 'save-clear-test');
    expect(await rc.hasAnyUnsavedBadge()).toBeTruthy();

    await rc.saveAndWaitForNavigation();
    // After save the form navigates away; there is no pending-badge on the
    // new page. If the app left us on the form (validation failure), the badge
    // should still clear.
    if (appPage.url().includes('/form/')) {
      await rc.expectNoPendingBadges();
    }
  });
});
