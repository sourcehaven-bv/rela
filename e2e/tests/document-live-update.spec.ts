import { test } from './fixtures';

/**
 * Document live-update tests — all skipped because the inline test project
 * in fixtures.ts does not configure `documents:` for any entity type. These
 * tests encode the intended behaviour of the documents panel so that the day
 * somebody adds a document to the inline project (e.g. to exercise the
 * documents feature end-to-end), the tests can be unskipped.
 *
 * The skip-reason is preserved from /frontend/e2e/document-live-update.spec.ts
 * (see RR-SG0LP).
 */

test.describe('Document Live Updates (disabled: no documents configured)', () => {
  test.skip('document updates when entity is modified via API', async () => {
    // Original behaviour: create ticket, open detail page, verify
    // DocumentsPanel shows initial content, update entity priority via API,
    // assert the document text reflects the new priority within ~10s of SSE.
    // Unskip once the inline fixtures.ts DATA_ENTRY_YAML adds a `documents:`
    // block with a command that prints entity properties.
  });

  test.skip('document shows cached badge when content is from cache', async () => {
    // Original behaviour: render twice, second render should hit the cache.
    // Unskip when documents are configured.
  });

  test.skip('refresh button forces document re-render', async () => {
    // Original behaviour: click the Refresh button in the documents panel
    // and assert the spinner resolves.
    // Unskip when documents are configured.
  });
});
