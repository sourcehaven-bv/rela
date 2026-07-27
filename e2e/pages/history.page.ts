import { type Locator, expect } from "@playwright/test";
import { BasePage } from "./base.page";

/** A comparison side as it appears in the URL and in the dropdowns: a version
 *  ordinal, or the `current` sentinel (the live entity). */
export type Side = number | "current";

function sideValue(s: Side): string {
  return String(s);
}

/** Page object for the entity history view (`/history/:type/:id`,
 *  HistoryView.vue). Selectors mirror the component's stable class names —
 *  `.timeline`, `.compare-select`, `.compare-caption`, `.prop-diff` — which are
 *  shared with RelationHistoryView, so the two page objects deliberately look
 *  alike. The compared pair is mirrored into `?base=`/`?target=` (TKT-YOHC3N),
 *  which is what makes a diff linkable. */
export class HistoryPage extends BasePage {
  /** Open the history view for an entity, optionally deep-linking a pair. */
  async open(type: string, id: string, selection?: { base?: Side; target?: Side }) {
    const params = new URLSearchParams();
    if (selection?.base !== undefined) params.set("base", sideValue(selection.base));
    if (selection?.target !== undefined) params.set("target", sideValue(selection.target));
    const qs = params.toString();
    await this.navigateTo(`/history/${type}/${id}${qs ? `?${qs}` : ""}`);
    await expect(this.page.locator(".history-view")).toBeVisible();
  }

  timelineItems(): Locator {
    return this.page.locator(".timeline .timeline-item");
  }

  async timelineCount(): Promise<number> {
    return this.timelineItems().count();
  }

  /** Wait until the timeline has at least `n` rows (create/update capture is
   *  asynchronous — the reconciliation sweep). Named `AtLeast` to match the
   *  relation page object, where a same-named exact-count waiter also exists. */
  async waitForTimelineAtLeast(n: number) {
    await expect
      .poll(async () => this.timelineCount(), { timeout: 20_000 })
      .toBeGreaterThanOrEqual(n);
  }

  /** Click a timeline row, which sets the BASE (before) side. */
  async selectVersion(version: number) {
    await this.page
      .locator(`.timeline-item[data-version="${version}"] .timeline-select`)
      .click();
  }

  baseSelect(): Locator {
    return this.page.locator(".compare-select").nth(0);
  }

  targetSelect(): Locator {
    return this.page.locator(".compare-select").nth(1);
  }

  /** Choose the two compared versions via the dropdowns. */
  async compare(base: Side, target: Side) {
    await this.baseSelect().selectOption(sideValue(base));
    await this.targetSelect().selectOption(sideValue(target));
  }

  /** The pair the dropdowns currently show. Reading the SELECT values (rather
   *  than the URL) is what catches a v-model type mismatch: a numeric side
   *  parsed as a string matches no <option> and leaves the control blank. */
  async selectedPair(): Promise<{ base: string; target: string }> {
    return {
      base: await this.baseSelect().inputValue(),
      target: await this.targetSelect().inputValue(),
    };
  }

  /** The `base → target` caption under the compare bar. */
  caption(): Locator {
    return this.page.locator(".compare-caption");
  }

  /** The view's root, visible once the route has rendered. */
  root(): Locator {
    return this.page.locator(".history-view");
  }

  /** Assert the view rendered (used after a raw navigation or a reload). */
  async expectLoaded() {
    await expect(this.root()).toBeVisible();
  }

  /** The timeline row for a version — `.selected` marks the base side. */
  timelineItem(version: number): Locator {
    return this.page.locator(`.timeline-item[data-version="${version}"]`);
  }

  /** Assert the timeline highlights `version` as the base side. */
  async expectSelectedVersion(version: number) {
    await expect(this.timelineItem(version)).toHaveClass(/selected/);
  }

  /** Assert no error state is rendered (malformed params must degrade
   *  silently to defaults rather than surface an error). */
  async expectNoError() {
    await expect(this.page.locator(".error-state")).toHaveCount(0);
  }

  /** Reload the page and wait for the view to come back. */
  async reload() {
    await this.page.reload();
    await this.expectLoaded();
  }

  /** Read one arbitrary query param off the current URL — used to assert that
   *  a selection write MERGED into the query rather than clobbering it. */
  queryParam(name: string): string | null {
    return new URL(this.page.url()).searchParams.get(name);
  }

  async swapSides() {
    await this.page.locator(".compare-swap").click();
  }

  /** The `?base=`/`?target=` params currently in the address bar. */
  urlSelection(): { base: string | null; target: string | null } {
    const params = new URL(this.page.url()).searchParams;
    return { base: params.get("base"), target: params.get("target") };
  }

  /** Wait until the URL reflects a specific pair (the write is a
   *  router.replace, so it lands a tick after the interaction). */
  async expectUrlSelection(base: Side, target: Side) {
    await expect
      .poll(() => this.urlSelection())
      .toEqual({ base: sideValue(base), target: sideValue(target) });
  }

  propDiffRows(): Locator {
    return this.page.locator(".prop-diff .prop-row");
  }

  async propDiffLabels(): Promise<string[]> {
    return this.page.locator(".prop-diff .prop-row .prop-label").allTextContents();
  }

  async restoreVersion(version: number) {
    await this.page
      .locator(`.timeline-item[data-version="${version}"]`)
      .locator("button", { hasText: "Restore" })
      .click();
  }

  /** The "version history is not available for this deployment" state (fsstore). */
  async expectUnsupported() {
    await expect(
      this.page.locator(".loading-state", { hasText: /not available/i }),
    ).toBeVisible();
  }
}
