import { test, expect } from './fixtures';
import { DocumentPage } from '../pages';

/**
 * E2E for the Edit button on the standalone document view (TKT-9QNHN).
 *
 * Two docs in the fixture:
 *   - feature_summary  -> has edit: { form: feature, label: "Edit feature" }
 *   - feature_readonly -> no edit block (button must NOT render)
 */

// Single source of truth for fixture-coupled identifiers, so renaming any
// of these in fixtures.ts surfaces as a focused failure here rather than
// scattered string drift across asserts.
const DOC_WITH_EDIT = 'feature_summary';
const DOC_NO_EDIT = 'feature_readonly';
const FEATURE_ID = 'FEAT-001';
const EDIT_FORM = 'feature_edit';
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

    await doc.editButton(EDIT_LABEL).click();
    await appPage.waitForURL(new RegExp(`/form/${EDIT_FORM}/${FEATURE_ID}`), { timeout: 10000 });

    // Parse return_to via URLSearchParams rather than asserting on the exact
    // percent-encoded form, so the test isn't coupled to encoding rules
    // (RR-BS0O4).
    const formURL = new URL(appPage.url());
    const returnTo = formURL.searchParams.get('return_to');
    expect(returnTo).toBe(`/document/${DOC_WITH_EDIT}/${FEATURE_ID}`);
  });

  test('saving the form returns to the document URL', async ({ appPage }) => {
    const doc = new DocumentPage(appPage);
    await doc.navigateToDocument(DOC_WITH_EDIT, FEATURE_ID);

    await doc.editButton(EDIT_LABEL).click();
    await appPage.waitForURL(new RegExp(`/form/${EDIT_FORM}/${FEATURE_ID}`), { timeout: 10000 });

    // Submit the form unchanged. Edit mode allows a no-op save.
    const saveResponse = appPage.waitForResponse(
      (r) => r.url().includes(`/api/v1/features/${FEATURE_ID}`) && r.request().method() === 'PATCH',
    );
    await appPage.locator('button[type="submit"]').first().click();
    await saveResponse;

    await appPage.waitForURL(new RegExp(`/document/${DOC_WITH_EDIT}/${FEATURE_ID}`), { timeout: 10000 });
    await expect(appPage.locator('.document-body')).toBeVisible({ timeout: 15000 });
  });

  test('no Edit button when edit block is absent', async ({ appPage }) => {
    const doc = new DocumentPage(appPage);
    await doc.navigateToDocument(DOC_NO_EDIT, FEATURE_ID);

    // No Edit button (label only renders when configured).
    await expect(doc.editButton(EDIT_LABEL)).toHaveCount(0);

    // Refresh remains. (Back is absent on a deep-linked load — TKT-JIEKC's
    // BackButton only renders when return_to/from is present; not what
    // we're testing here.)
    await expect(appPage.getByRole('button', { name: 'Refresh' })).toBeVisible();
  });

  test('mobile viewport: title stacks above the action row, edit + refresh visible', async ({ appPage }) => {
    // The 3-column desktop layout (Back | title | actions) collapses
    // ungracefully below 768px without the @media rule — title wraps
    // under itself and the action buttons land on the wrong side. This
    // pins the fixed layout.
    await appPage.setViewportSize({ width: 375, height: 700 });

    const doc = new DocumentPage(appPage);
    await doc.navigateToDocument(DOC_WITH_EDIT, FEATURE_ID);

    const h1 = appPage.locator('.page-header h1');
    const headerRight = appPage.locator('.page-header .header-right');

    await expect(h1).toBeVisible();
    await expect(headerRight).toBeVisible();

    const titleBox = await h1.boundingBox();
    const rightBox = await headerRight.boundingBox();
    if (!titleBox || !rightBox) {
      throw new Error('header bounding boxes unavailable');
    }

    // Title is on its own row above the action row (deep-link has no Back
    // button so the action row collapses to just .header-right).
    expect(titleBox.y + titleBox.height).toBeLessThanOrEqual(rightBox.y);

    // Both action buttons remain tappable on the action row.
    await expect(doc.editButton(EDIT_LABEL)).toBeVisible();
    await expect(headerRight.getByRole('button', { name: 'Refresh' })).toBeVisible();
  });
});
