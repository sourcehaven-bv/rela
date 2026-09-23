import { test, expect } from './fixtures';
import { SidebarPage } from '../pages';
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

/**
 * Query-driven entity lists in the sidebar (TKT-PEKL8L).
 *
 * The group is added by rewriting data-entry.yaml while the page is open, so
 * the config reload's `refresh` event is what makes it appear. That is the
 * path an operator takes, and it pins that the sidebar refetches its
 * definition without a page reload.
 */
test.describe('Sidebar entities entries', () => {
  const GROUP = 'In flight';

  /** Prepend an `entities:` group to the fixture's navigation. */
  const addEntitiesGroup = (projectDir: string) => {
    const file = join(projectDir, 'data-entry.yaml');
    const yaml = readFileSync(file, 'utf8');
    expect(yaml).toContain('\nnavigation:\n');
    writeFileSync(
      file,
      yaml.replace(
        '\nnavigation:\n',
        `\nnavigation:\n  - group: "${GROUP}"\n    items:\n      - entities: feature\n        query_scope: in_flight\n`,
      ),
    );
  };

  test('lists the scope, follows writes and links to the entity', async ({ appPage, testProject, api }) => {
    const sidebar = new SidebarPage(appPage);
    await sidebar.expectNavLinkVisible('Features');
    await sidebar.expectGroupAbsent(GROUP);

    addEntitiesGroup(testProject);
    // Seed: FEAT-003 "Export Data" is the only in_progress feature.
    await sidebar.expectGroupLinks(GROUP, ['Export Data']);

    // A write that brings an entity into scope shows up without a reload.
    await api.updateEntity('features', 'FEAT-002', { status: 'in_progress' });
    await sidebar.expectGroupLinks(GROUP, ['Dashboard Analytics', 'Export Data']);

    // And one that takes the last entities out hides the whole group.
    await api.updateEntity('features', 'FEAT-002', { status: 'done' });
    await api.updateEntity('features', 'FEAT-003', { status: 'done' });
    await sidebar.expectGroupAbsent(GROUP);

    await api.updateEntity('features', 'FEAT-003', { status: 'in_progress' });
    await sidebar.openGroupLink(GROUP, 'Export Data', 'FEAT-003');
  });
});
