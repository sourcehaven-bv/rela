import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * Page object for the pending-indicator framework (TKT-TFSNBY).
 *
 * Exists because the load-bearing guarantees are GEOMETRIC — a button whose
 * width does not change between its resting and pending states — and jsdom
 * has no layout engine, so the unit tests can only assert the structural
 * preconditions (both labels present, the inactive one hidden by
 * `visibility` rather than removed). Measuring the real box needs a real
 * browser.
 */
export class PendingPage extends BasePage {
  readonly activityBar: Locator;

  constructor(page: Page) {
    super(page);
    this.activityBar = page.locator('[data-testid="activity-bar"]');
  }

  /** A PendingButton by its resting label. */
  pendingButton(label: string): Locator {
    return this.page.locator(`.pending-button:has(.pending-button__label:text-is("${label}"))`);
  }

  /** Measured width of a button's border box, in CSS pixels. */
  async buttonWidth(label: string): Promise<number> {
    const box = await this.pendingButton(label).boundingBox();
    if (!box) throw new Error(`pending button "${label}" has no bounding box`);
    return box.width;
  }

  /**
   * Both label states must be in the DOM at all times — that is what
   * reserves the width. Removing either (v-if / display:none) collapses
   * the reservation and the button resizes mid-click.
   */
  async expectBothLabelsPresent(restingLabel: string, pendingLabel: string) {
    const button = this.pendingButton(restingLabel);
    await expect(button.locator(`.pending-button__label:text-is("${restingLabel}")`)).toHaveCount(1);
    await expect(button.locator(`.pending-button__label:text-is("${pendingLabel}")`)).toHaveCount(1);
  }

  /**
   * The pending label is present but not visible at rest. Playwright's
   * `toBeHidden` is satisfied by `visibility: hidden`, which is precisely
   * the mechanism under test — the element still occupies its box.
   */
  async expectPendingLabelReservedButHidden(restingLabel: string, pendingLabel: string) {
    const label = this.pendingButton(restingLabel).locator(
      `.pending-button__label:text-is("${pendingLabel}")`
    );
    await expect(label).toBeHidden();
    // Non-zero box = still contributing to the parent's width.
    const box = await label.boundingBox();
    expect(box?.width ?? 0).toBeGreaterThan(0);
  }

  /**
   * Width of an individual label box inside a pending button.
   *
   * The reservation only works if the HIDDEN label still contributes its
   * own (wider) box to the shared grid cell. Measuring the labels
   * separately is what distinguishes a real reservation from a button
   * that merely happens to be wide enough already.
   */
  async labelWidth(restingLabel: string, which: string): Promise<number> {
    const box = await this.pendingButton(restingLabel)
      .locator(`.pending-button__label:text-is("${which}")`)
      .boundingBox();
    if (!box) throw new Error(`label "${which}" has no bounding box`);
    return box.width;
  }

  /** Navigate a burst of routes back-to-back, without settling between. */
  async rapidNavigate(paths: string[]) {
    for (const path of paths) {
      await this.navigateTo(path);
    }
    await this.page.waitForLoadState('networkidle');
  }

  async expectActivityBarHidden() {
    await expect(this.activityBar).not.toHaveClass(/activity-bar--visible/);
  }
}
