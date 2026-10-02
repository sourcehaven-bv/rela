import { type Page, type Locator, expect } from "@playwright/test";

export class BasePage {
  readonly page: Page;
  readonly sidebar: Locator;
  /** Every toast in rela-components' RlToastHost. */
  readonly toastContainer: Locator;
  /** Toasts in the danger tone, which uiStore.error() raises. */
  readonly errorToast: Locator;
  /** The Back button rendered by <BackButton>. See TKT-JIEKC. A view
   *  only renders this when the URL carries a safe ?return_to= or
   *  ?from=<list-id>; otherwise the locator matches nothing. Keys on
   *  data-testid so the locator doesn't accidentally miss a button
   *  whose label was derived from a list title (e.g. "All Tickets"
   *  with no literal "Back" in the text). */
  readonly backButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.sidebar = page.locator(".sidebar, nav");
    this.toastContainer = page.locator('.rl-toast');
    this.errorToast = page.locator('.rl-toast--danger');
    this.backButton = page.locator('[data-testid="back-button"]').first();
  }

  /** Set a filter control to one value, or an enum filter to several.
   *  Relation filters are native selects; enum filters are multi-selects whose
   *  panel holds a checkbox per value, named by the value's label. '' clears
   *  the filter. */
  async pickFilterOption(control: Locator, value: string | string[] | { index: number }) {
    const native = control.locator("select");
    if (await native.count()) {
      await native.selectOption(value as never);
      return;
    }
    await control.locator('button[aria-haspopup="true"]').click();
    const boxes = this.page.locator(".rl-panel--options").getByRole("checkbox");
    for (const box of await boxes.all()) {
      if (await box.isChecked()) await box.uncheck();
    }
    if (!Array.isArray(value) && typeof value === "object") {
      await boxes.nth(value.index).check();
    } else {
      for (const v of [value].flat().filter(Boolean)) {
        await this.page
          .locator(".rl-panel--options")
          .getByRole("checkbox", { name: new RegExp(`^${v}$`, "i") })
          .check();
      }
    }
    await this.page.keyboard.press("Escape");
  }

  /** Assert the Back button is visible. */
  async expectBackButtonVisible() {
    await expect(this.backButton).toBeVisible();
  }

  /** Assert no Back button is rendered. */
  async expectNoBackButton() {
    await expect(this.page.locator('[data-testid="back-button"]')).toHaveCount(
      0,
    );
  }

  /** Click the Back button and wait for navigation to leave the current URL. */
  async clickBack() {
    const startUrl = this.page.url();
    await this.backButton.click();
    // waitForURL's predicate receives a URL object; compare via href so
    // the equality check is against the same string format as page.url().
    await this.page.waitForURL((url) => url.href !== startUrl);
  }

  async navigateTo(path: string) {
    // SPA routes are served at the root path.
    const currentUrl = this.page.url();
    const baseUrl = new URL(currentUrl).origin;
    const fullPath = path.startsWith("/") ? path : `/${path}`;
    await this.page.goto(`${baseUrl}${fullPath}`);
    await this.page.waitForLoadState("domcontentloaded");
  }

  async clickNavLink(name: string) {
    await this.page.getByRole("link", { name }).click();
    await this.page.waitForLoadState("domcontentloaded");
  }

  /** Click a sidebar link by visible label and wait for the target page's heading. */
  async clickSidebarLink(
    label: string,
    expectedHeading: string | RegExp = label,
  ) {
    await this.page
      .getByRole("link", { name: new RegExp(label) })
      .first()
      .click();
    const matcher =
      expectedHeading instanceof RegExp
        ? expectedHeading
        : new RegExp(expectedHeading);
    await expect(
      this.page.locator("h1").filter({ hasText: matcher }),
    ).toBeVisible();
  }

  async expectNavLinkVisible(label: string) {
    await expect(this.page.getByRole("link", { name: label })).toBeVisible();
  }

  async waitForToast(message?: string) {
    if (message) {
      await expect(this.page.getByText(message)).toBeVisible();
    } else {
      await expect(this.toastContainer.first()).toBeVisible();
    }
  }

  async expectHeading(text: string) {
    await expect(
      this.page.locator("h1, h2").filter({ hasText: text }).first(),
    ).toBeVisible();
  }

  async expectUrl(pattern: RegExp | string) {
    if (typeof pattern === "string") {
      await expect(this.page).toHaveURL(new RegExp(pattern));
    } else {
      await expect(this.page).toHaveURL(pattern);
    }
  }

  /**
   * Computed `animation-name` of whatever RlSpinner actually animates.
   *
   * Reads the inner element rather than the component's root: the root is
   * positioned by the consumer and the animation sits on the glyph inside
   * it, which is precisely why the placement transform can no longer be
   * clobbered by a rotation keyframe.
   */
  async spinnerAnimationName(spinner: Locator): Promise<string> {
    return spinner
      .locator(".rl-spinner__svg")
      .evaluate((el) => getComputedStyle(el).animationName);
  }

  /**
   * Which animation RlSpinner is running: its rotation, its reduced-motion
   * fade, or none at all.
   *
   * Matched on a PREFIX rather than the literal keyframe name. The component
   * declares its keyframes inside a scoped `<style>`, so Vue rewrites
   * `rl-spin` to `rl-spin-<hash>` at build time — asserting the bare name
   * passes in dev and fails against the production bundle the e2e suite runs.
   */
  async spinnerAnimationKind(
    spinner: Locator,
  ): Promise<"spin" | "pulse" | "none"> {
    const name = await this.spinnerAnimationName(spinner);
    if (name === "none" || name === "") return "none";
    if (name.startsWith("rl-spinner-pulse")) return "pulse";
    if (name.startsWith("rl-spin")) return "spin";
    throw new Error(`unrecognised spinner animation: ${name}`);
  }

  /**
   * Vertical distance between a spinner's centre and its offset parent's,
   * sampled mid-animation.
   *
   * The regression this guards: a rotation keyframe applied to the same
   * element that carries the centring `transform` REPLACES it, dropping the
   * spinner half its height the instant it animates. RlSpinner makes that
   * structurally impossible by animating an inner element, and this measures
   * the outcome so the guarantee survives a change to how it does that.
   */
  async centringOffsetMidAnimation(spinner: Locator): Promise<number> {
    // Typed as HTMLElement for `offsetParent`: RlSpinner's root is a <span>,
    // and only the glyph inside it is an SVG.
    return spinner.evaluate((el: HTMLElement) => {
      const svg = el.querySelector(".rl-spinner__svg");
      // Park part-way through a rotation, where a clobbered translate shows
      // up as a vertical displacement.
      const anim = svg?.getAnimations()[0];
      if (anim) {
        anim.currentTime = 150;
        anim.pause();
      }
      const parent = el.offsetParent ?? el.parentElement!;
      const p = parent.getBoundingClientRect();
      const s = el.getBoundingClientRect();
      return s.top + s.height / 2 - (p.top + p.height / 2);
    });
  }

  async waitForSpinnerToDisappear() {
    const spinner = this.page.locator(".rl-spinner, .rl-status-region--pending");
    if (await spinner.isVisible({ timeout: 100 }).catch(() => false)) {
      await expect(spinner).not.toBeVisible();
    }
  }

  /** Click a form's save/submit button and wait for the SPA to navigate
   *  away from the current edit form. The form save path issues an entity
   *  PATCH followed by per-relation POST/DELETE/PATCH calls and ends in a
   *  router.push back to the entity detail; both FormPage and
   *  RelationCardsPage share that same contract, so this helper is the
   *  single source of truth for the navigation predicate. */
  async submitFormAndWaitForNavigation(submitButton: Locator) {
    const submitVisible = await submitButton
      .waitFor({ state: "visible", timeout: 2000 })
      .then(() => true)
      .catch(() => false);
    if (submitVisible) {
      await Promise.all([
        this.page.waitForURL((url) => !url.pathname.includes("/form/"), {
          timeout: 10000,
        }),
        submitButton.click(),
      ]);
      return;
    }
    // TKT-E6094: in autosave mode there is no explicit submit; the form
    // saves continuously. Blur to flush pending input, wait for any
    // in-flight or queued autosave PATCH to land, then navigate back.
    await this.page.evaluate(() =>
      (document.activeElement as HTMLElement | null)?.blur(),
    );
    // Wait for at least one PATCH on the current entity (the autosave),
    // bounded so a clean form (no edits) doesn't hang.
    await this.page
      .waitForResponse(
        (r) => r.url().includes("/api/v1/") && r.request().method() === "PATCH",
        { timeout: 2000 },
      )
      .catch(() => {});
    await Promise.all([
      this.page.waitForURL((url) => !url.pathname.includes("/form/"), {
        timeout: 10000,
      }),
      this.page.goBack(),
    ]);
  }

  async confirmDialog() {
    this.page.once("dialog", (dialog) => dialog.accept());
  }

  async dismissDialog() {
    this.page.once("dialog", (dialog) => dialog.dismiss());
  }
}
