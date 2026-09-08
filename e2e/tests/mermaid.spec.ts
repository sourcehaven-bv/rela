import { test, expect } from './fixtures';
import { EntityPage } from '../pages';

/**
 * Mermaid diagrams in entity body content, rendered in a REAL browser through
 * the real mermaid library and the real sanitizer.
 *
 * The unit tests in frontend/src/utils/markdown.test.ts mock mermaid.render()
 * — necessary for the attack cases, since strict mermaid never emits a
 * <script> — so this is the one place that checks what mermaid ACTUALLY emits
 * survives sanitization. Flowchart labels are XHTML inside <foreignObject>, and
 * every single-pass DOMPurify configuration silently discards them while the
 * shapes still render: the diagram looks fine and reads as empty boxes. jsdom
 * cannot run mermaid at all (no layout engine — getBBox), so this cannot be a
 * unit test.
 */
test.describe('Mermaid diagrams in entity content', () => {
  let featureId: string | null = null;

  test.beforeEach(async ({ api }) => {
    const feature = await api.createEntity('features', {
      properties: {
        title: 'Mermaid Render Feature',
        description: 'Renders a flowchart from body content',
        status: 'draft',
        priority: 'high',
      },
      content: [
        '```mermaid',
        'graph TD',
        '  A["Line1<br/>Line2"] --> B[Done]',
        '  B --> C["<img src=x onerror=alert(1)>"]',
        '```',
      ].join('\n'),
    });
    featureId = feature.id;
  });

  test.afterEach(async ({ api }) => {
    if (featureId) {
      await api.deleteEntity('features', featureId).catch(() => {});
      featureId = null;
    }
  });

  test('renders flowchart labels with their markup intact', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', featureId!);
    await entity.waitForMermaidDiagram();

    const labels = (await entity.mermaidLabelTexts()).join('\n');
    expect(labels).toContain('Line1');
    expect(labels).toContain('Line2');
    expect(labels).toContain('Done');

    // The label SUBTREE must survive, not just its text: text alone also
    // survives the broken configurations. The .nodeLabel wrapper is mermaid's
    // styling hook; the <br> is what keeps a multi-line node from collapsing
    // into one run-together word.
    expect(await entity.mermaidLabelElementCount()).toBeGreaterThan(0);
    expect(await entity.mermaidLabelLineBreakCount()).toBeGreaterThan(0);
  });

  test('strips executable content from a hostile label', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', featureId!);
    await entity.waitForMermaidDiagram();

    expect(await entity.mermaidEventHandlerAttributes()).toEqual([]);
    // Strict mermaid strips the handler but still emits the <img>; the
    // sanitizer must remove the element itself.
    expect(await entity.mermaidEmbeddedMediaCount()).toBe(0);
  });
});
