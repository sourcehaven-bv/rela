import { type Page, expect } from '@playwright/test';

/** Small wrapper around page.request for test API setup that asserts success. */
export class ApiClient {
  constructor(private page: Page, private serverUrl: string) {}

  async createEntity(plural: string, properties: Record<string, unknown>): Promise<{ id: string }> {
    const resp = await this.page.request.post(`${this.serverUrl}/api/v1/${plural}`, {
      data: { properties },
    });
    expect(resp.ok(), `POST /api/v1/${plural} failed: ${resp.status()}`).toBeTruthy();
    const entity = await resp.json();
    expect(entity.id, 'created entity should have an id').toBeTruthy();
    return entity;
  }

  async createRelation(fromPlural: string, fromId: string, relation: string, toId: string): Promise<void> {
    const resp = await this.page.request.post(
      `${this.serverUrl}/api/v1/${fromPlural}/${fromId}/relations/${relation}`,
      { data: { id: toId } },
    );
    expect(resp.ok(), `create relation ${relation} failed: ${resp.status()}`).toBeTruthy();
  }
}
