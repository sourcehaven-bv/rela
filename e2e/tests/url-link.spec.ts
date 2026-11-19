import { test, expect } from './fixtures';
import { EntityPage, FormPage, KanbanPage, ListPage } from '../pages';

// TKT-0101IO4: a string property with `display: {link: true}` renders through
// the url widget on every surface. An http(s) value is a link that opens in a
// new tab without an opener; any other scheme stays text.
test.describe('URL link display', () => {
  const pdf = 'https://example.com/invoices/INV-1.pdf';
  const hostile = 'javascript:alert(1)';

  test('links an https value on the detail page, list, kanban card and form', async ({ appPage, api }) => {
    const bug = await api.createEntity('bugs', {
      properties: { title: 'Linked bug', status: 'draft', doc_url: pdf },
    });

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('bug', bug.id);
    await entity.expectUrlProperty(pdf, pdf);

    const list = new ListPage(appPage);
    await list.navigateToList('bug_links');
    await list.expectUrlCellInRow(bug.id, pdf, pdf);

    // The card is itself a link that opens the side panel; a click on the
    // url field opens the URL instead.
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-link-board');
    await kanban.expectCardUrl('Linked bug', pdf, pdf);
    expect(await kanban.clickCardUrl('Linked bug', pdf)).toBe(pdf);

    const form = new FormPage(appPage);
    await form.navigateToEditForm('bug', bug.id);
    await form.expectUrlInput('doc_url');
    await form.expectFieldValue('doc_url', pdf);
  });

  test('keeps a javascript: value as text, and its form still saves', async ({ appPage, api }) => {
    const bug = await api.createEntity('bugs', {
      properties: { title: 'Hostile bug', status: 'draft', doc_url: hostile },
    });

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('bug', bug.id);
    await entity.expectUrlProperty(hostile, null);

    const list = new ListPage(appPage);
    await list.navigateToList('bug_links');
    await list.expectUrlCellInRow(bug.id, hostile, null);

    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-link-board');
    await kanban.expectCardUrl('Hostile bug', hostile, null);

    // A stored value that is not a URL must not block the form's save.
    const form = new FormPage(appPage);
    await form.navigateToEditForm('bug', bug.id);
    await form.fillTitle('Hostile bug renamed');
    await form.saveAndWaitForPatch('bugs', bug.id);
    await expect
      .poll(async () => (await api.getEntity('bugs', bug.id)).properties.title)
      .toBe('Hostile bug renamed');
    expect((await api.getEntity('bugs', bug.id)).properties.doc_url).toBe(hostile);
  });
});
