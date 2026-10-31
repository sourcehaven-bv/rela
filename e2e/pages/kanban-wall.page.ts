import { type Locator, type Page } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * A wall board (`layout: wall`, TKT-0VBQW0H): a pile of loose cards and the
 * groups below it. Reads what each holds, and groups cards by dropping one
 * on the middle of another, waiting for the board to show the result.
 */
export class KanbanWallPage extends BasePage {
  constructor(page: Page) {
    super(page);
  }

  /** The titles of the cards on a plain wall, in order. */
  async wallTitles(): Promise<string[]> {
    return this.page.locator(".rl-board-column[data-section-id='__wall__'] .kanban-card .card-title").allInnerTexts();
  }

  /** The titles of the loose cards, in order. */
  async pileTitles(): Promise<string[]> {
    return this.pile().locator('.kanban-card .card-title').allInnerTexts();
  }

  /** Each group's name and its cards' titles, in order. */
  async groups(): Promise<Array<{ name: string; cards: string[] }>> {
    const columns = this.page.locator(".rl-board-column:not([data-section-id='__other__'])");
    const out: Array<{ name: string; cards: string[] }> = [];
    for (const column of await columns.all()) {
      const name = await column.locator('input.wall-heading__title').inputValue();
      const cards = await column.locator('.kanban-card .card-title').allInnerTexts();
      out.push({ name, cards: cards.map((c) => c.trim()) });
    }
    return out;
  }

  /** Drop the card titled `title` on the middle of the card titled `target`. */
  async dropOnto(title: string, target: string) {
    const settled = this.page.waitForResponse(
      (r) => /\/api\/v1\/[^/]+\/[^/]+$/.test(r.url()) && r.request().method() === 'PATCH',
    );
    await this.drag(this.card(title), this.card(target), 0.5);
    await settled;
  }

  /** Drop the card titled `title` on the right edge of the card titled `target`. */
  async dropAfter(title: string, target: string) {
    await this.whileOrdering(() => this.drag(this.card(title), this.card(target), 0.9));
  }

  /** Drag the group named `name` by its handle onto the left half of the group named `target`. */
  async moveGroupBefore(name: string, target: string) {
    const handle = this.group(name).getByRole('button', { name: 'Drag to move' });
    await this.whileOrdering(() => this.drag(handle, this.group(target), 0.25));
  }

  private group(name: string): Locator {
    return this.page.locator('.rl-board-column').filter({
      has: this.page.locator(`input.wall-heading__title[value="${name}"]`),
    });
  }

  /** Run a gesture and wait for the reorder it sends to the server. */
  private async whileOrdering(gesture: () => Promise<void>) {
    const moved = this.page.waitForResponse(
      (r) => /\/relations\/[^/]+\/[^/]+$/.test(r.url()) && r.request().method() === 'PATCH',
    );
    await gesture();
    await moved;
  }

  // Stepped moves, as in KanbanPage: a native drag needs a few pointer moves
  // to start, and one jump to the target can race it. `across` is where on
  // the target, from its left edge, the drop lands.
  private async drag(from: Locator, onto: Locator, across: number) {
    // Cards slide to their new place after every change; a drag measured
    // mid-slide aims at where a card was. Wait for the slides, not for the
    // infinite wobble of a hovered drop target.
    await this.page.waitForFunction(() =>
      document.getAnimations().every((a) => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity),
    );
    const a = (await from.boundingBox())!;
    const b = (await onto.boundingBox())!;
    const start = { x: a.x + a.width / 2, y: a.y + a.height / 2 };
    const end = { x: b.x + b.width * across, y: b.y + b.height / 2 };
    await this.page.mouse.move(start.x, start.y);
    await this.page.mouse.down();
    await this.page.mouse.move(start.x + 5, start.y + 5, { steps: 2 });
    await this.page.mouse.move(end.x, end.y, { steps: 10 });
    await this.page.mouse.up();
  }

  private pile(): Locator {
    return this.page.locator(".rl-board-column[data-section-id='__other__']");
  }

  private card(title: string): Locator {
    return this.page.locator('.rl-board-card').filter({ hasText: title });
  }
}
