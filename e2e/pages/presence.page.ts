import { type Page, type Locator, type APIRequestContext, expect, request } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * Presence avatars and the claim badge (TKT-IMBF7U), as one user sees them
 * on the entity page, a list and a kanban board.
 */
export class PresencePage extends BasePage {
  readonly avatars: Locator;
  readonly detailBadge: Locator;

  constructor(page: Page) {
    super(page);
    this.avatars = page.locator('.detail-header [data-testid="presence-avatars"]');
    this.detailBadge = page.locator('.detail-header .lease-badge');
  }

  async open(path: string) {
    await this.page.goto(path);
    await this.page.waitForLoadState('domcontentloaded');
  }

  async openEntity(type: string, id: string) {
    await this.open(`/entity/${type}/${id}`);
    await expect(this.page.locator('.detail-header')).toBeVisible({ timeout: 15_000 });
  }

  avatarOf(user: string, tool = 'data-entry'): Locator {
    return this.avatars.locator(`.presence-avatar[title="${user} (${tool})"]`);
  }

  async expectViewer(user: string, timeout = 15_000) {
    await expect(this.avatarOf(user)).toBeVisible({ timeout });
  }

  async expectNoViewer(user: string, timeout = 30_000) {
    await expect(this.avatarOf(user)).toHaveCount(0, { timeout });
  }

  /** No avatar of `user` in any tool. */
  async expectNotListed(user: string) {
    await expect(this.avatars.locator(`.presence-avatar[title^="${user}"]`)).toHaveCount(0);
  }

  async expectDetailBadge(text: string, timeout = 15_000) {
    // The visible text only; the badge also holds a visually hidden status.
    await expect(this.detailBadge.locator('.lease-badge__text')).toHaveText(text, { timeout });
  }

  async expectNoDetailBadge(timeout = 15_000) {
    await expect(this.detailBadge).toHaveCount(0, { timeout });
  }

  private listBadge(rowText: string): Locator {
    return this.page.getByRole('row').filter({ hasText: rowText }).locator('.lease-badge');
  }

  async expectListBadge(rowText: string, text: string, timeout = 15_000) {
    await expect(this.listBadge(rowText).locator('.lease-badge__text')).toHaveText(text, { timeout });
  }

  async expectNoListBadge(rowText: string, timeout = 15_000) {
    await expect(this.page.getByRole('row').filter({ hasText: rowText }).first()).toBeVisible({ timeout });
    await expect(this.listBadge(rowText)).toHaveCount(0, { timeout });
  }

  private cardBadge(cardText: string): Locator {
    return this.page.locator('.kanban-card', { hasText: cardText }).locator('.lease-badge');
  }

  async expectCardBadge(cardText: string, text: string, timeout = 15_000) {
    await expect(this.cardBadge(cardText).locator('.lease-badge__text')).toHaveText(text, { timeout });
  }

  async expectNoCardBadge(cardText: string, timeout = 15_000) {
    await expect(this.page.locator('.kanban-card', { hasText: cardText }).first()).toBeVisible({ timeout });
    await expect(this.cardBadge(cardText)).toHaveCount(0, { timeout });
  }
}

/**
 * Claims and releases leases the way a script or agent does: a plain HTTP
 * client with no cookie and no Origin, identified by the proxy header.
 */
export class LeaseClient {
  private constructor(private ctx: APIRequestContext) {}

  static async as(serverUrl: string, header: string, user: string): Promise<LeaseClient> {
    const ctx = await request.newContext({ baseURL: serverUrl, extraHTTPHeaders: { [header]: user } });
    return new LeaseClient(ctx);
  }

  /** Claims the entity and returns the secret lease id. */
  async claim(plural: string, id: string, body: { label?: string; status?: string } = {}): Promise<string> {
    const resp = await this.ctx.post(`/api/v1/${plural}/${id}/_lease`, { data: body });
    expect(resp.status(), `claim ${id}: ${await resp.text()}`).toBe(201);
    return (await resp.json()).lease_id;
  }

  async release(leaseId: string) {
    const resp = await this.ctx.post('/api/v1/_leases/release', { data: { lease_id: leaseId } });
    expect(resp.status(), `release: ${await resp.text()}`).toBe(204);
  }

  async dispose() {
    await this.ctx.dispose();
  }
}
