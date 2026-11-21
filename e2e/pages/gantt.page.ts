import { type Page, type Locator } from '@playwright/test';
import { BasePage } from './base.page';

export class GanttPage extends BasePage {
  readonly chart: Locator;
  readonly nowButton: Locator;
  readonly laterButton: Locator;
  readonly earlierButton: Locator;

  constructor(page: Page) {
    super(page);
    this.chart = page.locator('.gantt-view .chart');
    this.nowButton = page.getByRole('button', { name: 'Now', exact: true });
    this.laterButton = page.getByRole('button', { name: 'Later' });
    this.earlierButton = page.getByRole('button', { name: 'Earlier' });
  }

  async navigateToGantt(id: string) {
    await this.navigateTo(`/gantt/${id}`);
    await this.chart.waitFor();
  }

  /** The ids of the rows, in the order the chart shows them. */
  rowIds(): Promise<(string | null)[]> {
    return this.page
      .locator('.gantt-view .row[data-node-id]')
      .evaluateAll((els) => els.map((el) => el.getAttribute('data-node-id')));
  }

  /** The titles of the rows, in the order the chart shows them. */
  rowTitles(): Promise<string[]> {
    return this.page
      .locator('.gantt-view .row[data-node-id] .cell-tree')
      .evaluateAll((els) => els.map((el) => (el.textContent ?? '').trim()));
  }

  /** The tree cell of the row whose title contains `text`. */
  rowTreeCellWithText(text: string): Locator {
    return this.page.locator('.gantt-view .row[data-node-id] .cell-tree').filter({ hasText: text });
  }

  /** The chart's rows. */
  get rows(): Locator {
    return this.page.locator('.gantt-view .row[data-node-id]');
  }

  zoomButton(zoom: 'quarter' | 'month' | 'week'): Locator {
    return this.page.locator('.zoom-seg button', { hasText: zoom });
  }

  scrollLeft(): Promise<number> {
    return this.chart.evaluate((el) => el.scrollLeft);
  }

  setScrollLeft(px: number): Promise<void> {
    return this.chart.evaluate((el, v) => {
      el.scrollLeft = v;
    }, px);
  }

  /** Viewport x of an element's left edge, relative to the chart's own left edge. */
  async xInChart(locator: Locator): Promise<number> {
    const [box, chart] = await Promise.all([locator.boundingBox(), this.chart.boundingBox()]);
    if (!box || !chart) throw new Error('element not laid out');
    return box.x - chart.x;
  }

  /** The visible span of the today line, relative to the chart's left edge. */
  todayLine(): Locator {
    return this.page.locator('.axis .today-flag');
  }

  barLabel(nodeId: string): Locator {
    return this.page.locator(`.row[data-node-id="${nodeId}"] .bar-name`);
  }

  bar(nodeId: string): Locator {
    return this.page.locator(`.row[data-node-id="${nodeId}"] .bar`);
  }

  rowTreeCell(nodeId: string): Locator {
    return this.page.locator(`.row[data-node-id="${nodeId}"] .cell-tree`);
  }

  /** A draggable bar's control: the move window or one of its edges. */
  dragHandle(nodeId: string, mode: 'start' | 'move' | 'end'): Locator {
    return this.page.locator(`.row[data-node-id="${nodeId}"] [data-testid="gantt-drag-${mode}"]`);
  }

  /** Drags a handle sideways by dx pixels with the mouse, in steps. */
  async dragBy(handle: Locator, dx: number): Promise<void> {
    const box = await handle.boundingBox();
    if (!box) throw new Error('handle not laid out');
    const x = box.x + box.width / 2;
    const y = box.y + box.height / 2;
    await this.page.mouse.move(x, y);
    await this.page.mouse.down();
    await this.page.mouse.move(x + dx, y, { steps: 8 });
    await this.page.mouse.up();
  }
}
