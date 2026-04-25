import { test, expect, SEED } from './fixtures';
import { RelationCardsPage, FormPage } from '../pages';

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
  // Backend-contract tests for the relations endpoints (response shape with
  // direction:incoming, grouping under the inverse name) live in Go:
  // internal/dataentry/api_v1_test.go::TestV1GetRelationType_IncomingReturnsEdgeWithMeta
  // and TestV1EntityRelations_GroupsIncomingUnderInverseName. The specs
  // below cover only what requires a real browser.

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

  test('non-cards picker lists linked source entities with direction: incoming', async ({ appPage }) => {
    // The 'implements' relation goes task -> feature. On the FEAT-001 form
    // we configure a non-cards widget with direction: incoming labelled
    // "Implemented by". TASK-001 implements FEAT-001 in the seed, so the
    // widget must render TASK-001 as a linked entity.
    //
    // Backend contract is covered by the Go test referenced above; here we
    // only verify that the SPA renders the reverse-direction picker value.
    const form = new FormPage(appPage);
    await form.navigateToEditForm('feature', SEED.features.authentication);
    const picker = form.relationPickerByLabel('Implemented by');
    await expect(picker).toBeVisible();
    await expect(form.pickerTileByText(picker, 'Write unit tests')).toBeVisible();
  });
});
