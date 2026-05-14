import { test, expect } from './fixtures';
import { DocumentPage, FormPage } from '../pages';

/**
 * E2E for the Edit button on the standalone document view (TKT-9QNHN).
 *
 * Two docs in the fixture:
 *   - feature_summary  -> has edit: { form: feature_edit, label: "Edit feature" }
 *   - feature_readonly -> no edit block (button must NOT render)
 */

// Single source of truth for fixture-coupled identifiers, so renaming any
// of these in fixtures.ts surfaces as a focused failure here rather than
// scattered string drift across asserts.
const DOC_WITH_EDIT = 'feature_summary';
const DOC_NO_EDIT = 'feature_readonly';
const FEATURE_ID = 'FEAT-001';
const EDIT_FORM = 'feature';
const EDIT_LABEL = 'Edit feature';

test.describe('Document view: Edit button', () => {
  test('renders Edit button when edit block is configured', async ({ appPage }) => {
    const doc = new DocumentPage(appPage);
    await doc.navigateToDocument(DOC_WITH_EDIT, FEATURE_ID);

    await expect(doc.editButton(EDIT_LABEL)).toBeVisible();
  });

  test('Edit button click navigates to the form with return_to set to the document path', async ({
    appPage,
  }) => {
    const doc = new DocumentPage(appPage);
    await doc.navigateToDocument(DOC_WITH_EDIT, FEATURE_ID);

    const form = new FormPage(appPage);
    await doc.editButton(EDIT_LABEL).click();
    await form.expectAtFormUrl(EDIT_FORM, FEATURE_ID);

    // Parse return_to via URLSearchParams rather than asserting on the exact
    // percent-encoded form, so the test isn't coupled to encoding rules
    // (RR-BS0O4).
    const returnTo = form.readReturnTo();
    expect(returnTo).toBe(`/document/${DOC_WITH_EDIT}/${FEATURE_ID}`);
  });

  test('navigating back from the form returns to the document URL', async ({ appPage }) => {
    // Was: 'saving the form returns to the document URL'. After
    // TKT-E6094 there's no explicit Save in edit mode — autosave runs
    // continuously and never triggers navigation. The user goes back
    // explicitly; this test verifies the return_to round-trips.
    const doc = new DocumentPage(appPage);
    await doc.navigateToDocument(DOC_WITH_EDIT, FEATURE_ID);

    const form = new FormPage(appPage);
    await doc.editButton(EDIT_LABEL).click();
    await form.expectAtFormUrl(EDIT_FORM, FEATURE_ID);

    // No edits — go back. The route guard's commitImmediately returns
    // settled:true on a quiet queue, so navigation proceeds silently.
    await appPage.goBack();

    await doc.expectAtDocumentUrl(DOC_WITH_EDIT, FEATURE_ID);
  });

  test('no Edit button when edit block is absent', async ({ appPage }) => {
    const doc = new DocumentPage(appPage);
    await doc.navigateToDocument(DOC_NO_EDIT, FEATURE_ID);

    // No Edit button (label only renders when configured).
    await expect(doc.editButton(EDIT_LABEL)).toHaveCount(0);

    // Refresh remains. (Back is absent on a deep-linked load — TKT-JIEKC's
    // BackButton only renders when return_to/from is present; not what
    // we're testing here.)
    await expect(doc.refreshButton).toBeVisible();
  });

  test('mobile viewport: title stacks above the action row, edit + refresh visible', async ({ appPage }) => {
    // The 3-column desktop layout (Back | title | actions) collapses
    // ungracefully below 768px without the @media rule — title wraps
    // under itself and the action buttons land on the wrong side. This
    // pins the fixed layout.
    await appPage.setViewportSize({ width: 375, height: 700 });

    const doc = new DocumentPage(appPage);
    await doc.navigateToDocument(DOC_WITH_EDIT, FEATURE_ID);

    await expect(doc.title).toBeVisible();
    await expect(doc.headerRight).toBeVisible();

    const titleBox = await doc.title.boundingBox();
    const rightBox = await doc.headerRight.boundingBox();
    if (!titleBox || !rightBox) {
      throw new Error('header bounding boxes unavailable');
    }

    // Title is on its own row above the action row (deep-link has no Back
    // button so the action row collapses to just .header-right).
    expect(titleBox.y + titleBox.height).toBeLessThanOrEqual(rightBox.y);

    // Both action buttons remain tappable on the action row.
    await expect(doc.editButton(EDIT_LABEL)).toBeVisible();
    await expect(doc.refreshButton).toBeVisible();
  });
});
