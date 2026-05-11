import { test, expect } from './fixtures';
import { EntityPage } from '../pages';

// Verifies the implicit entity-reference resolution that turns bare-ID code
// spans in markdown content into in-app links (TKT-747O). The server attaches
// a `mentions` map to the view response and the SPA markdown renderer
// rewrites matching `<code>` nodes to `<a>` nodes with the target's title.
test.describe('Entity reference resolution', () => {
  let originId: string | null = null;
  let targetId: string | null = null;

  test.beforeEach(async ({ api }) => {
    const target = await api.createEntity('features', {
      properties: {
        title: 'Resolved Target',
        description: 'Target referenced by code-span ID from another entity',
        status: 'draft',
        priority: 'medium',
      },
    });
    targetId = target.id;

    const origin = await api.createEntity('features', {
      properties: {
        title: 'Origin With Reference',
        description: 'Content references the target via code span',
        status: 'draft',
        priority: 'medium',
      },
      // Body content carries a bare-ID code span; the renderer must rewrite
      // it to an <a> linking to the target's detail page.
      content: `see \`${target.id}\` for the dependency`,
    });
    originId = origin.id;
  });

  test.afterEach(async ({ api }) => {
    if (originId) {
      await api.deleteEntity('features', originId).catch(() => {});
      originId = null;
    }
    if (targetId) {
      await api.deleteEntity('features', targetId).catch(() => {});
      targetId = null;
    }
  });

  test('renders a known-ID code span as a titled in-app link', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', originId!);

    const link = entity.contentEntityRefLink('feature', targetId!);
    await expect(link).toBeVisible();
    await expect(link).toHaveText('Resolved Target');
    // Negative: a known-ID code span must NOT remain as <code> in the
    // rendered output (the rewrite either fires or it doesn't —
    // partial rewrites are not allowed).
    await expect(entity.contentCodeSpan(targetId!)).toHaveCount(0);
  });

  test('clicking the rendered link navigates to the target entity', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', originId!);

    await entity.clickContentEntityRef('feature', targetId!);
    // The target's title appears on the destination page.
    expect(await entity.containsText('Resolved Target')).toBeTruthy();
  });

  test('unknown-ID code spans remain as <code> (no link)', async ({ api, appPage }) => {
    // A separate entity whose content references an ID that doesn't
    // resolve. The renderer must leave the code span untouched.
    const unknownRefOwner = await api.createEntity('features', {
      properties: {
        title: 'Unknown Reference Owner',
        description: 'Code span references a nonexistent ID',
        status: 'draft',
        priority: 'medium',
      },
      content: 'see `FEAT-DOES-NOT-EXIST` for a non-resolution',
    });

    try {
      const entity = new EntityPage(appPage);
      await entity.navigateToEntity('feature', unknownRefOwner.id);
      // The code span is rendered as <code>...</code>, not as an <a>.
      await expect(entity.contentCodeSpan('FEAT-DOES-NOT-EXIST')).toHaveCount(1);
      await expect(
        entity.contentEntityRefLink('feature', 'FEAT-DOES-NOT-EXIST'),
      ).toHaveCount(0);
    } finally {
      await api.deleteEntity('features', unknownRefOwner.id).catch(() => {});
    }
  });

  test('IDs inside fenced code blocks are not linkified', async ({ api, appPage }) => {
    // Even a known ID, when it appears inside a fenced code block, must
    // stay as code — the resolver only fires on inline code spans.
    const fencedOwner = await api.createEntity('features', {
      properties: {
        title: 'Fenced Reference Owner',
        description: 'Known ID inside a fenced code block',
        status: 'draft',
        priority: 'medium',
      },
      content: `prose\n\n\`\`\`\n${targetId}\n\`\`\`\n`,
    });

    try {
      const entity = new EntityPage(appPage);
      await entity.navigateToEntity('feature', fencedOwner.id);
      // Fenced blocks render as <pre><code>; the rewrite must not fire.
      await expect(
        entity.contentEntityRefLink('feature', targetId!),
      ).toHaveCount(0);
    } finally {
      await api.deleteEntity('features', fencedOwner.id).catch(() => {});
    }
  });
});
