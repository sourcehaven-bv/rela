import * as fs from 'fs';
import * as path from 'path';
import { test, expect, type ApiHelpers } from './fixtures';
import { EntityPage, ListPage } from '../pages';

/**
 * Detect git-crypt encrypted entity files at the storage layer and
 * surface them in the data-entry UI as inaccessible records. The spec
 * does NOT exercise actual git-crypt — it writes a file whose first
 * bytes are the git-crypt magic header (\x00GITCRYPT\x00), which is
 * what fsstore looks at. The watcher picks the change up, the in-memory
 * index reflects the new state, and the SPA renders lock indicators.
 */

const GIT_CRYPT_MAGIC = Buffer.from([0x00, 0x47, 0x49, 0x54, 0x43, 0x52, 0x59, 0x50, 0x54, 0x00]);
const ENCRYPTED_BLOB = Buffer.concat([GIT_CRYPT_MAGIC, Buffer.from('opaque ciphertext bytes')]);

/**
 * Overwrite an entity file in the test project's working tree with a
 * git-crypt blob and wait for the file watcher to reconcile it. The
 * watcher debounce is short, but allow a generous timeout because CI
 * filesystem latency varies.
 */
async function encryptEntityFile(
  testProject: string,
  api: ApiHelpers,
  plural: string,
  id: string,
): Promise<void> {
  fs.writeFileSync(path.join(testProject, 'entities', plural, `${id}.md`), ENCRYPTED_BLOB);
  await expect
    .poll(
      async () => {
        const e = await api.getEntity(plural, id).catch(() => null);
        return e?.inaccessible?.length ?? 0;
      },
      { timeout: 10_000, message: `entity ${id} did not flip to inaccessible` },
    )
    .toBeGreaterThan(0);
}

test.describe('git-crypt encrypted entities', () => {
  test('list view marks encrypted cells with a lock indicator', async ({
    appPage,
    api,
    testProject,
  }) => {
    await encryptEntityFile(testProject, api, 'features', 'FEAT-001');

    const list = new ListPage(appPage);
    await list.navigateToList('features');

    // The list config shows three columns (title/status/priority), each
    // of which is locked for the encrypted entity.
    expect(await list.lockedCellsInRow('FEAT-001')).toBe(3);
  });

  test('detail view shows the inaccessible banner and locks every property', async ({
    appPage,
    api,
    testProject,
  }) => {
    await encryptEntityFile(testProject, api, 'features', 'FEAT-001');

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', 'FEAT-001');
    await entity.expectInaccessibleBanner();

    // The feature schema declares title/status/description/priority —
    // all four render as locked placeholders on a fully-encrypted file.
    expect(await entity.lockedPropertyCount()).toBe(4);
  });

  test('encrypted entity hides the Edit button', async ({ appPage, api, testProject }) => {
    await encryptEntityFile(testProject, api, 'features', 'FEAT-001');

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', 'FEAT-001');
    expect(await entity.hasEditButton()).toBe(false);
  });

  // PATCH-rejection is verified server-side in
  // internal/dataentry/inaccessible_test.go:TestV1UpdateEntity_RejectsInaccessible.
  // Per e2e/tests/AGENTS.md ("API-only assertions belong in Go") we do
  // not duplicate that here.

  test('decrypt-in-place reloads the entity as readable', async ({
    appPage,
    api,
    testProject,
  }) => {
    // Start encrypted.
    await encryptEntityFile(testProject, api, 'features', 'FEAT-001');

    // Now drop a fresh cleartext file in place — simulates `git-crypt
    // unlock` rewriting the working tree. The watcher reloads the
    // entity with full Properties.
    const cleartext = `---
id: FEAT-001
type: feature
title: Decrypted feature
status: approved
priority: high
---

Now readable.
`;
    fs.writeFileSync(
      path.join(testProject, 'entities', 'features', 'FEAT-001.md'),
      cleartext,
    );
    await expect
      .poll(
        async () => {
          const e = await api.getEntity('features', 'FEAT-001');
          return e.inaccessible?.length ?? 0;
        },
        { timeout: 10_000, message: 'entity FEAT-001 did not flip back to readable' },
      )
      .toBe(0);

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', 'FEAT-001');
    expect(await entity.containsText('Decrypted feature')).toBe(true);
    expect(await entity.hasEditButton()).toBe(true);
  });
});
