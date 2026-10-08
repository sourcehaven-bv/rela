import { type Locator, type Page, expect } from '@playwright/test';
import { BasePage } from './base.page';

type Placement = 'before' | 'after';

/**
 * Rows and cards in relation order (TKT-RCRUWZ): a list tab's table, a
 * board tab's columns, and a table section on an entity page. Each reports
 * its items in the order shown, and moves one by pointer or keyboard,
 * waiting for the move request to land.
 */
export class RelationOrderPage extends BasePage {
  constructor(page: Page) {
    super(page);
  }

  /** The titles of a list tab's rows, top to bottom. */
  async listTitles(): Promise<string[]> {
    return this.page.locator('.rl-table-row .rl-table-row__primary').allInnerTexts();
  }

  /** The titles of one board column's cards, top to bottom. */
  async columnTitles(column: string): Promise<string[]> {
    return this.column(column).locator('.kanban-card .card-title').allInnerTexts();
  }

  /** The titles of a table section's rows, top to bottom, as their move handles name them. */
  async sectionTitles(heading: string): Promise<string[]> {
    const rows = this.section(heading).locator('tbody tr');
    const handles = await rows.locator('.ordered-row__handle').evaluateAll((els) =>
      els.map((el) => el.getAttribute('aria-label') ?? ''),
    );
    return handles.map((label) => label.replace(/^Move /, ''));
  }

  /** Move a list row a place up or down with its handle's arrow keys. */
  async stepListRow(title: string, key: 'ArrowUp' | 'ArrowDown') {
    await this.whileMoving(async () => {
      const handle = this.page.getByRole('button', { name: `Move ${title}` });
      await handle.focus();
      await handle.press(key);
    });
  }

  /** Move a section row a place up or down with its handle's arrow keys. */
  async stepSectionRow(heading: string, title: string, key: 'ArrowUp' | 'ArrowDown') {
    await this.whileMoving(async () => {
      const handle = this.section(heading).getByRole('button', { name: `Move ${title}` });
      await handle.focus();
      await handle.press(key);
    });
  }

  /** Drag a list row by its handle onto the top or bottom half of another. */
  async dragListRow(title: string, target: string, placement: Placement) {
    const handle = this.page.getByRole('button', { name: `Move ${title}` });
    const row = this.page.locator('.rl-table-row').filter({ hasText: target });
    await this.whileMoving(() => this.drag(handle, row, placement));
  }

  /** Drag a card onto the top or bottom half of another card. */
  async dragCard(title: string, target: string, placement: Placement) {
    const card = this.page.locator('.kanban-card').filter({ hasText: title });
    const onto = this.page.locator('.kanban-card').filter({ hasText: target });
    await this.whileMoving(() => this.drag(card, onto, placement));
  }

  async expectHandles(count: number) {
    await expect(this.page.getByRole('button', { name: /^Move / })).toHaveCount(count);
  }

  private column(name: string): Locator {
    return this.page.locator('.rl-board-column').filter({
      has: this.page.locator('.rl-section-heading', { hasText: name }),
    });
  }

  private section(heading: string): Locator {
    return this.page.locator('.view-section', { has: this.page.getByRole('heading', { name: heading }) });
  }

  /** Run a gesture and wait for the move it sends to the server. */
  private async whileMoving(gesture: () => Promise<void>) {
    const moved = this.page.waitForResponse(
      (r) => /\/relations\/[^/]+\/[^/]+$/.test(r.url()) && r.request().method() === 'PATCH',
    );
    await gesture();
    expect((await moved).status()).toBe(204);
  }

  // Stepped moves, as in KanbanPage: a native drag needs a few pointer moves
  // to start, and one jump to the target can race it.
  private async drag(from: Locator, onto: Locator, placement: Placement) {
    const a = (await from.boundingBox())!;
    const b = (await onto.boundingBox())!;
    const start = { x: a.x + a.width / 2, y: a.y + a.height / 2 };
    const end = { x: b.x + b.width / 2, y: b.y + (placement === 'before' ? b.height * 0.25 : b.height * 0.75) };
    await this.page.mouse.move(start.x, start.y);
    await this.page.mouse.down();
    await this.page.mouse.move(start.x + 5, start.y + 5, { steps: 2 });
    await this.page.mouse.move(end.x, end.y, { steps: 10 });
    await this.page.mouse.up();
  }
}
