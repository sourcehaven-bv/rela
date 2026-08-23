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

  /**
   * Computed `animation-name` for a bare element carrying `className`,
   * under whatever media emulation is currently active.
   */
  async animationNameFor(className: string): Promise<string> {
    return this.page.evaluate((cls) => {
      const el = document.createElement('div');
      el.className = cls;
      document.body.appendChild(el);
      const name = getComputedStyle(el).animationName;
      el.remove();
      return name;
    }, className);
  }

  /**
   * Vertical distance between an animated, absolutely-centred spinner's
   * centre and its offset parent's centre, measured MID-ANIMATION.
   *
   * Built as a standalone fixture rather than driven through the real
   * relation search, because the property under test is pure CSS geometry:
   * does the rotation keyframe clobber the centring transform? Reproducing
   * the exact `.search-spinner` box inside a positioned parent isolates
   * that without depending on search timing or seed data.
   */
  async animatedCentringOffset(): Promise<number> {
    return this.page.evaluate(() => {
      const parent = document.createElement('div');
      parent.style.cssText = 'position:relative;height:60px;width:200px';
      const spinner = document.createElement('div');
      // Mirrors .search-spinner's box and centring strategy.
      spinner.style.cssText =
        'position:absolute;right:12px;top:50%;margin-top:-8px;width:16px;' +
        'height:16px;box-sizing:border-box;border:2px solid #000;border-radius:50%;' +
        'animation:spin 0.6s linear infinite';
      parent.appendChild(spinner);
      document.body.appendChild(parent);
      // Sample part-way through a rotation, where a clobbered translate
      // shows up as a vertical displacement.
      const anim = spinner.getAnimations()[0];
      if (anim) {
        anim.currentTime = 150;
        anim.pause();
      }
      const p = parent.getBoundingClientRect();
      const s = spinner.getBoundingClientRect();
      const offset = s.top + s.height / 2 - (p.top + p.height / 2);
      parent.remove();
      return offset;
    });
  }

  async expectActivityBarHidden() {
    await expect(this.activityBar).not.toHaveClass(/activity-bar--visible/);
  }
}
