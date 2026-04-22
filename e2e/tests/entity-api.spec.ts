import { test, expect } from './fixtures';

/**
 * REST API v1 contract tests. The UI CRUD tests live in crud.spec.ts; these
 * lock in the shape and behaviour of /api/v1/{plural} without going through
 * any UI.
 */

test.describe('Entity CRUD via API', () => {
  test.describe('Features', () => {
    test('create', async ({ api }) => {
      const created = await api.createEntity('features', {
        properties: { title: 'API Create', status: 'draft', priority: 'medium' },
      });
      try {
        expect(created.id).toMatch(/^FEAT-\d+$/);
        expect(created.type).toBe('feature');
        expect(created.properties.title).toBe('API Create');
      } finally {
        await api.deleteEntity('features', created.id).catch(() => {});
      }
    });

    test('read', async ({ api }) => {
      const created = await api.createEntity('features', {
        properties: { title: 'API Read', status: 'draft', priority: 'low' },
      });
      try {
        const fetched = await api.getEntity('features', created.id);
        expect(fetched.id).toBe(created.id);
        expect(fetched.properties.title).toBe('API Read');
      } finally {
        await api.deleteEntity('features', created.id).catch(() => {});
      }
    });

    test('update', async ({ api }) => {
      const created = await api.createEntity('features', {
        properties: { title: 'API Update', status: 'draft', priority: 'low' },
      });
      try {
        const updated = await api.updateEntity('features', created.id, {
          title: 'API Update (modified)',
          priority: 'high',
        });
        expect(updated.properties.title).toBe('API Update (modified)');
        expect(updated.properties.priority).toBe('high');
      } finally {
        await api.deleteEntity('features', created.id).catch(() => {});
      }
    });

    test('delete', async ({ api }) => {
      const created = await api.createEntity('features', {
        properties: { title: 'API Delete', status: 'draft', priority: 'low' },
      });
      await api.deleteEntity('features', created.id);
      await expect(api.getEntity('features', created.id)).rejects.toThrow();
    });

    test('list returns pagination envelope', async ({ api }) => {
      const page = await api.listEntities('features');
      expect(Array.isArray(page.data)).toBeTruthy();
      expect(page.meta).toBeDefined();
      expect(typeof page.meta.total).toBe('number');
    });

    test('list filters by property', async ({ api }) => {
      const page = await api.listEntities('features', 'filter[status]=draft');
      for (const e of page.data) {
        expect(e.properties.status).toBe('draft');
      }
    });
  });

  test.describe('Relations', () => {
    test('create a relation between two features', async ({ api }) => {
      const a = await api.createEntity('features', {
        properties: { title: 'Rel A', status: 'draft', priority: 'low' },
      });
      const b = await api.createEntity('features', {
        properties: { title: 'Rel B', status: 'draft', priority: 'low' },
      });
      try {
        await api.createRelation('features', a.id, 'blocks', b.id);
        const resp = await api.rawRequest('GET', `features/${a.id}/relations/blocks`);
        const rels = (await resp.json()) as Array<{ id: string }>;
        expect(rels.some((r) => r.id === b.id)).toBeTruthy();
      } finally {
        await api.deleteEntity('features', a.id).catch(() => {});
        await api.deleteEntity('features', b.id).catch(() => {});
      }
    });
  });
});
