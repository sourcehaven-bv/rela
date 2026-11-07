import { test as base, expect, contextsAs, serveWithPrincipalHeader } from './fixtures';
import { CollabBodyPage } from '../pages';

// Two people edit one entity body at once (TKT-CJL604). Each is a browser
// context the proxy header identifies, as in presence.spec.ts.

const ALICE = 'alice@example.com';
const BOB = 'bob@example.com';
const BODY = 'Shared opening line.\n\nSecond paragraph.';

/** Stored markdown as the editor shows it: one line per block, unescaped. */
function asText(markdown: string): string {
  return markdown
    .replace(/\\(.)/g, '$1')
    .split('\n')
    .filter((line) => line.trim() !== '')
    .join('\n');
}

const test = base.extend({
  serverUrl: async ({ testProject, serverBinary }, use, testInfo) => {
    await serveWithPrincipalHeader(serverBinary, testProject, testInfo, use);
  },
});

test.describe('co-editing an entity body', () => {
  test.describe.configure({ timeout: 120_000 });

  test('two editors converge and one of them saves the shared body', async ({ browser, serverUrl, api }) => {
    const created = await api.createEntity('features', {
      properties: { title: 'Co-edited body' },
      content: BODY,
    });
    const people = contextsAs(browser, serverUrl);
    try {
      const alice = new CollabBodyPage(await people.as(ALICE), created.id);
      const bob = new CollabBodyPage(await people.as(BOB), created.id);
      await alice.openEditor('feature', created.id);
      await bob.openEditor('feature', created.id);

      // Each sees the other's labelled cursor.
      await alice.caretAfter('Second paragraph.');
      await bob.caretAfter('Shared opening line.');
      await expect(alice.peerCursor(BOB)).toBeVisible({ timeout: 15_000 });
      await expect(bob.peerCursor(ALICE)).toBeVisible({ timeout: 15_000 });

      // Typing in different paragraphs reaches the other editor.
      await alice.type(' Alice was here.');
      await expect.poll(() => bob.bodyText()).toContain('Alice was here.');
      await bob.type(' Bob too.');
      await expect.poll(() => alice.bodyText()).toContain('Bob too.');

      // Both type on the same line at the same time, one at its start and
      // one at its end; neither loses a character.
      await alice.caretAfter('Bob too.');
      await bob.caretAtStartOf('Bob too.');
      await Promise.all([alice.type(' [a1a2a3]'), bob.type('[b1b2b3] ')]);
      for (const page of [alice, bob]) {
        await expect.poll(() => page.bodyText()).toContain('[b1b2b3] Shared opening line. Bob too. [a1a2a3]');
      }
      await expect.poll(async () => (await alice.bodyText()) === (await bob.bodyText())).toBe(true);

      // The stored body is the shared document, character for character.
      const shared = await alice.bodyText();
      expect(shared).toContain('Second paragraph. Alice was here.');
      await expect
        .poll(async () => asText(await api.getContent('features', created.id)), { timeout: 15_000 })
        .toBe(shared);

      // Only the elected saver writes. The other would save only if the room
      // had not saved its latest text by the end of its fallback delay.
      await alice.waitPastSaveFallback();
      expect(
        [alice.saveCount, bob.saveCount].filter((n) => n > 0),
        `saves: alice ${alice.saveCount}, bob ${bob.saveCount}`,
      ).toHaveLength(1);
    } finally {
      await people.close();
    }
  });
});
