import { type Page, type Locator, expect } from "@playwright/test";
import { BasePage } from "./base.page";

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

  /**
   * A label-swapping button by its resting label.
   *
   * The mechanism now lives in the shared component library's `RlButton`,
   * which reserves the width exactly as rela's own button did — both labels
   * stacked in one grid cell, the inactive one hidden by `visibility`. So
   * this object measures the same geometry through the library's class names.
   */
  pendingButton(label: string): Locator {
    return this.page.locator(
      `.rl-button:has(.rl-button__label:text-is("${label}"))`,
    );
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
    await expect(
      button.locator(`.rl-button__label:text-is("${restingLabel}")`),
    ).toHaveCount(1);
    await expect(
      button.locator(`.rl-button__label:text-is("${pendingLabel}")`),
    ).toHaveCount(1);
  }

  /**
   * The pending label is present but not visible at rest. Playwright's
   * `toBeHidden` is satisfied by `visibility: hidden`, which is precisely
   * the mechanism under test — the element still occupies its box.
   */
  async expectPendingLabelReservedButHidden(
    restingLabel: string,
    pendingLabel: string,
  ) {
    const label = this.pendingButton(restingLabel).locator(
      `.rl-button__label:text-is("${pendingLabel}")`,
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
      .locator(`.rl-button__label:text-is("${which}")`)
      .boundingBox();
    if (!box) throw new Error(`label "${which}" has no bounding box`);
    return box.width;
  }

  /**
   * The resting label is still the visible one, i.e. the pending gate never
   * elapsed.
   *
   * Asserted on the LABELS rather than on a state attribute on the button.
   * `rl-button--loading` and `aria-busy` track the raw `loading` prop, so they
   * are set for the whole request however short it is; what the gate governs is
   * which of the two stacked labels is hidden. That is also the thing the user
   * would actually see swap.
   */
  async expectLabelNeverSwapped(restingLabel: string, pendingLabel: string) {
    const button = this.pendingButton(restingLabel);
    await expect(
      button.locator(`.rl-button__label:text-is("${restingLabel}")`),
    ).toBeVisible();
    await expect(
      button.locator(`.rl-button__label:text-is("${pendingLabel}")`),
    ).toBeHidden();
  }

  /** Navigate a burst of routes back-to-back, without settling between. */
  async rapidNavigate(paths: string[]) {
    for (const path of paths) {
      await this.navigateTo(path);
    }
    await this.page.waitForLoadState("networkidle");
  }

  /**
   * Open /analyze with its response held, run `body` while the cold-load
   * spinner is on screen, then release.
   *
   * The spinner marks a COLD load, so it is on screen only while a request
   * is outstanding — a few frames against a local server. Stalling the
   * response is what makes the element measurable at all; racing the real
   * request gives a test that passes by luck.
   */
  async withColdLoadSpinner(body: (spinner: Locator) => Promise<void>) {
    const pattern = "**/api/v1/_analyze**";
    /*
     * The request is never answered — the spinner is what is under test, not
     * the analysis. Deliberately NOT "hold the route, then continue on the way
     * out": unrouting while a handler is still parked leaves it with a dead
     * route and throws `Route is already handled!`, which fails the test after
     * its assertions have already passed.
     */
    await this.page.route(pattern, () => {});
    try {
      // Not navigateTo: that waits for domcontentloaded, and the point is to
      // observe the page mid-flight. The origin is derived the same way,
      // because no baseURL is configured.
      const origin = new URL(this.page.url()).origin;
      await this.page.goto(`${origin}/analyze`, { waitUntil: "commit" });
      const spinner = this.page.locator(".rl-status-region--pending .rl-spinner");
      await expect(spinner).toBeVisible();
      await body(spinner);
    } finally {
      await this.page.unroute(pattern);
    }
  }

  async expectActivityBarHidden() {
    await expect(this.activityBar).not.toHaveClass(/activity-bar--visible/);
  }
}
